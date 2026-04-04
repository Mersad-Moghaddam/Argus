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
	"net/netip"
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
	monitors ports.MonitorStore
	alerts   ports.AlertChannelStore
	outbox   ports.OutboxStore
	service  *application.Service
	client   *asynq.Client
	notifier ports.Notifier
	logger   *observability.LogStore
}

func NewProcessor(monitors ports.MonitorStore, alerts ports.AlertChannelStore, outbox ports.OutboxStore, service *application.Service, client *asynq.Client, notifier ports.Notifier, logger *observability.LogStore) *Processor {
	return &Processor{monitors: monitors, alerts: alerts, outbox: outbox, service: service, client: client, notifier: notifier, logger: logger}
}
func (p *Processor) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(TypeEnqueueDueChecks, p.HandleEnqueueDueChecks)
	mux.HandleFunc(TypeCheckWebsite, p.HandleCheckWebsite)
	mux.HandleFunc(TypeDispatchOutbox, p.HandleDispatchOutbox)
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
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "down", resp.StatusCode, latency, "non-successful status code"
	}
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
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("invalid host")
	}
	if strings.EqualFold(host, "169.254.169.254") || strings.HasPrefix(host, "metadata.google.internal") {
		return fmt.Errorf("blocked metadata endpoint")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}
		if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
			return fmt.Errorf("target resolves to private address")
		}
	}
	return nil
}

var _ = strconv.Itoa
