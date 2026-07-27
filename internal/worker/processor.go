package worker

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"argus/internal/application"
	"argus/internal/domain"
	"argus/internal/domain/ports"
	"argus/internal/models"
	"argus/internal/observability"
	"github.com/hibiken/asynq"
)

type Processor struct {
	monitors            ports.MonitorStore
	routes              ports.RouteStore
	alerts              ports.AlertChannelStore
	outbox              ports.OutboxStore
	service             *application.Service
	client              *asynq.Client
	notifier            ports.Notifier
	logger              *observability.LogStore
	evaluator           *RouteEvaluator
	routeBatchSize      int
	routeTimeoutCeiling time.Duration
	routeRetention      time.Duration
	pruneBatchSize      int
}

func NewProcessor(monitors ports.MonitorStore, routes ports.RouteStore, alerts ports.AlertChannelStore, outbox ports.OutboxStore, service *application.Service, client *asynq.Client, notifier ports.Notifier, logger *observability.LogStore, routeBatchSize int, timeoutCeiling, retention time.Duration, pruneBatchSize int) *Processor {
	if routeBatchSize <= 0 {
		routeBatchSize = 500
	}
	if pruneBatchSize <= 0 {
		pruneBatchSize = 5000
	}
	return &Processor{monitors: monitors, routes: routes, alerts: alerts, outbox: outbox, service: service, client: client, notifier: notifier, logger: logger,
		evaluator: NewRouteEvaluator(), routeBatchSize: routeBatchSize, routeTimeoutCeiling: timeoutCeiling, routeRetention: retention, pruneBatchSize: pruneBatchSize}
}
func (p *Processor) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(TypeEnqueueDueChecks, p.HandleEnqueueDueChecks)
	mux.HandleFunc(TypeCheckWebsite, p.HandleCheckWebsite)
	mux.HandleFunc(TypeDispatchOutbox, p.HandleDispatchOutbox)
	mux.HandleFunc(TypeEnqueueDueRoutes, p.HandleEnqueueDueRoutes)
	mux.HandleFunc(TypeCheckRoute, p.HandleCheckRoute)
	mux.HandleFunc(TypeAggregateRoutes, p.HandleAggregateRoutes)
	mux.HandleFunc(TypePruneRouteChecks, p.HandlePruneRouteChecks)
}

func (p *Processor) HandleEnqueueDueRoutes(ctx context.Context, _ *asynq.Task) error {
	afterID := int64(0)
	now := time.Now().UTC()
	for {
		due, err := p.routes.ListDueRoutes(ctx, now, p.routeBatchSize, afterID)
		if err != nil {
			return err
		}
		if len(due) == 0 {
			return nil
		}
		for _, route := range due {
			afterID = route.ID
			task, taskErr := NewCheckRouteTask(route.ID)
			if taskErr != nil {
				return taskErr
			}
			uniqueFor := time.Duration(route.MonitorIntervalSecs) * time.Second
			if uniqueFor < time.Second {
				uniqueFor = time.Second
			}
			_, enqueueErr := p.client.EnqueueContext(ctx, task, asynq.Queue("default"), asynq.Unique(uniqueFor))
			if enqueueErr != nil && enqueueErr != asynq.ErrDuplicateTask {
				return enqueueErr
			}
		}
		if len(due) < p.routeBatchSize {
			return nil
		}
	}
}

func (p *Processor) HandleCheckRoute(ctx context.Context, task *asynq.Task) error {
	var payload CheckRoutePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	route, err := p.routes.GetRouteByID(ctx, payload.RouteID)
	if err != nil || route == nil {
		return err
	}
	if !route.Enabled {
		return nil
	}
	result := p.evaluator.Evaluate(ctx, *route, p.routeTimeoutCeiling)
	return p.service.ProcessRouteCheckResult(ctx, *route, result.Status, result.StatusCode, result.LatencyMS, result.FailureReason, result.Attempts, time.Now().UTC())
}

func (p *Processor) HandleAggregateRoutes(ctx context.Context, _ *asynq.Task) error {
	if err := p.routes.AggregateRouteMetrics(ctx, time.Now().UTC().Add(-24*time.Hour)); err != nil {
		return err
	}
	return p.routes.AggregateProjectMetrics(ctx)
}

func (p *Processor) HandlePruneRouteChecks(ctx context.Context, _ *asynq.Task) error {
	if p.routeRetention <= 0 {
		return nil
	}
	before := time.Now().UTC().Add(-p.routeRetention)
	for {
		deleted, err := p.routes.PruneRouteChecks(ctx, before, p.pruneBatchSize)
		if err != nil {
			return err
		}
		if deleted < int64(p.pruneBatchSize) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
}

func (p *Processor) HandleEnqueueDueChecks(ctx context.Context, _ *asynq.Task) error {
	afterID := int64(0)
	for {
		due, err := p.monitors.ListDue(ctx, time.Now().UTC(), 200, afterID)
		if err != nil {
			return err
		}
		if len(due) == 0 {
			break
		}
		for _, website := range due {
			afterID = website.ID
			t, err := NewCheckWebsiteTask(CheckWebsitePayload{WebsiteID: website.ID, URL: website.URL, HealthCheckURL: website.HealthCheckURL, Interval: website.CheckInterval})
			if err != nil {
				return err
			}
			_, enqueueErr := p.client.EnqueueContext(ctx, t, asynq.Queue("critical"), asynq.Unique(time.Duration(website.CheckInterval)*time.Second))
			if enqueueErr != nil && enqueueErr != asynq.ErrDuplicateTask {
				return enqueueErr
			}
		}
		if len(due) < 200 {
			break
		}
	}
	return nil
}

func (p *Processor) HandleCheckWebsite(ctx context.Context, task *asynq.Task) error {
	var payload CheckWebsitePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	website, err := p.monitors.GetByID(ctx, payload.WebsiteID)
	if err != nil || website == nil {
		return err
	}
	checkURL := payload.URL
	if payload.HealthCheckURL != nil {
		checkURL = *payload.HealthCheckURL
	}
	status, code, latency, reason := p.evaluate(ctx, website, checkURL)
	now := time.Now().UTC()
	next := now.Add(time.Duration(payload.Interval) * time.Second)
	if err = p.monitors.MarkChecked(ctx, payload.WebsiteID, status, code, latency, now, next); err != nil {
		return err
	}
	_ = p.monitors.RecordCheck(ctx, payload.WebsiteID, status, code, latency, reason, now)
	_ = p.service.ProcessIncidentTransition(ctx, payload.WebsiteID, payload.URL, status, reason, now)
	return nil
}

func (p *Processor) HandleDispatchOutbox(ctx context.Context, _ *asynq.Task) error {
	events, err := p.outbox.FetchPending(ctx, 100)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}
	channels, err := p.alerts.ListAlertChannels(ctx)
	if err != nil {
		return err
	}
	for _, evt := range events {
		if notifyErr := p.notifier.Notify(ctx, channels, evt.Payload); notifyErr != nil {
			_ = p.outbox.MarkFailed(ctx, evt.ID, notifyErr.Error())
			continue
		}
		_ = p.outbox.MarkProcessed(ctx, evt.ID)
	}
	return nil
}

func (p *Processor) evaluate(ctx context.Context, website *models.Website, target string) (string, int, int, string) {
	switch website.MonitorType {
	case domain.MonitorTypeKeyword:
		return p.checkKeyword(ctx, target, website.ExpectedKeyword)
	case domain.MonitorTypeHeartbeat:
		if website.LastHeartbeatAt == nil {
			return "down", 0, 0, "heartbeat never received"
		}
		grace := time.Duration(website.HeartbeatGraceSeconds) * time.Second
		if time.Since(*website.LastHeartbeatAt) > grace {
			return "down", 0, 0, "heartbeat stale"
		}
		return "up", 200, 0, ""
	case domain.MonitorTypeTLSExpiry:
		return p.checkTLS(target, website.TLSExpiryThresholdDays)
	default:
		return p.checkHTTP(ctx, target)
	}
}

func (p *Processor) checkHTTP(ctx context.Context, target string) (string, int, int, string) {
	if err := validateTarget(target); err != nil {
		return "down", 0, 0, err.Error()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return "down", 0, latency, err.Error()
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024*1024))
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return "up", resp.StatusCode, latency, ""
	}
	return "down", resp.StatusCode, latency, "non-successful status code"
}
func (p *Processor) checkKeyword(ctx context.Context, target string, keyword *string) (string, int, int, string) {
	if keyword == nil || *keyword == "" {
		return "down", 0, 0, "missing expected keyword"
	}
	if err := validateTarget(target); err != nil {
		return "down", 0, 0, err.Error()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return "down", 0, latency, err.Error()
	}
	defer resp.Body.Close()
	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text") && !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "json") {
		return "down", resp.StatusCode, latency, "unsupported content-type for keyword check"
	}
	reader := bufio.NewReader(io.LimitReader(resp.Body, 1024*1024))
	needle := *keyword
	buf := ""
	for {
		chunk, err := reader.ReadString('\n')
		buf += chunk
		if strings.Contains(buf, needle) {
			return "up", resp.StatusCode, latency, ""
		}
		if len(buf) > len(needle)*3 {
			buf = buf[len(buf)-len(needle)*2:]
		}
		if err != nil {
			break
		}
	}
	return "down", resp.StatusCode, latency, "expected keyword not found"
}
func (p *Processor) checkTLS(target string, thresholdDays int) (string, int, int, string) {
	parsed, err := url.Parse(target)
	if err != nil {
		return "down", 0, 0, "invalid URL"
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}
	start := time.Now()
	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", host, &tls.Config{ServerName: parsed.Hostname(), MinVersion: tls.VersionTLS12})
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return "down", 0, latency, err.Error()
	}
	defer conn.Close()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "down", 0, latency, "no peer certificates"
	}
	days := int(time.Until(certs[0].NotAfter).Hours() / 24)
	if days < thresholdDays {
		return "down", 200, latency, fmt.Sprintf("TLS certificate expires in %d days", days)
	}
	return "up", 200, latency, ""
}

func validateTarget(rawURL string) error {
	return validateTargetContext(context.Background(), rawURL)
}

var _ = strconv.Itoa
