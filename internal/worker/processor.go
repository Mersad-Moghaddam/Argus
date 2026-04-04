package worker

import (
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

	"argus/internal/models"
	"argus/internal/observability"
	"argus/internal/repository"
	"github.com/hibiken/asynq"
)

// Processor handles Asynq jobs.
type Processor struct {
	repo   repository.WebsiteRepository
	client *asynq.Client
	logger *observability.LogStore
}

func NewProcessor(repo repository.WebsiteRepository, client *asynq.Client, logger *observability.LogStore) *Processor {
	return &Processor{repo: repo, client: client, logger: logger}
}

func (p *Processor) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(TypeEnqueueDueChecks, p.HandleEnqueueDueChecks)
	mux.HandleFunc(TypeCheckWebsite, p.HandleCheckWebsite)
}

func (p *Processor) HandleEnqueueDueChecks(ctx context.Context, _ *asynq.Task) error {
	dueWebsites, err := p.repo.ListDue(ctx, time.Now().UTC())
	if err != nil {
		p.logger.Add("error", "worker", "fetch_due_websites_failed", "Failed to query due websites", nil, map[string]string{"error": err.Error()})
		return fmt.Errorf("list due websites: %w", err)
	}

	for _, website := range dueWebsites {
		task, taskErr := NewCheckWebsiteTask(CheckWebsitePayload{
			WebsiteID:      website.ID,
			URL:            website.URL,
			HealthCheckURL: website.HealthCheckURL,
			Interval:       website.CheckInterval,
		})
		if taskErr != nil {
			return taskErr
		}
		_, enqueueErr := p.client.EnqueueContext(ctx, task, asynq.Queue("critical"), asynq.Unique(time.Duration(website.CheckInterval)*time.Second))
		if enqueueErr != nil && enqueueErr != asynq.ErrDuplicateTask {
			return fmt.Errorf("enqueue website check task: %w", enqueueErr)
		}
	}
	return nil
}

func (p *Processor) HandleCheckWebsite(ctx context.Context, task *asynq.Task) error {
	var payload CheckWebsitePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal check website task payload: %w", err)
	}

	website, err := p.repo.GetByID(ctx, payload.WebsiteID)
	if err != nil {
		return err
	}
	if website == nil {
		return nil
	}

	status := "down"
	statusCode := 0
	latencyMS := 0
	failureReason := ""
	checkURL := payload.URL
	if payload.HealthCheckURL != nil {
		checkURL = *payload.HealthCheckURL
	}

	switch website.MonitorType {
	case models.MonitorTypeKeyword:
		status, statusCode, latencyMS, failureReason = p.checkKeyword(ctx, checkURL, website.ExpectedKeyword)
	case models.MonitorTypeHeartbeat:
		status, statusCode, latencyMS, failureReason = p.checkHeartbeat(website)
	case models.MonitorTypeTLSExpiry:
		status, statusCode, latencyMS, failureReason = p.checkTLSExpiry(checkURL, website.TLSExpiryThresholdDays)
	default:
		status, statusCode, latencyMS, failureReason = p.checkHTTP(ctx, checkURL)
	}

	checkedAt := time.Now().UTC()
	nextCheckAt := checkedAt.Add(time.Duration(payload.Interval) * time.Second)
	if err = p.repo.MarkChecked(ctx, payload.WebsiteID, status, statusCode, latencyMS, checkedAt, nextCheckAt); err != nil {
		return fmt.Errorf("mark website checked: %w", err)
	}
	_ = p.repo.RecordCheck(ctx, payload.WebsiteID, status, statusCode, latencyMS, failureReason, checkedAt)

	if err = p.handleIncidentsAndAlerts(ctx, payload.WebsiteID, payload.URL, status, failureReason, checkedAt); err != nil {
		p.logger.Add("error", "worker", "incident_alert_flow_failed", "Incident/alert flow failed", &payload.WebsiteID, map[string]string{"error": err.Error()})
	}

	details := map[string]string{"url": payload.URL, "checkTarget": checkURL, "status": status, "statusCode": strconv.Itoa(statusCode), "latencyMs": strconv.Itoa(latencyMS), "checkInterval": strconv.Itoa(payload.Interval)}
	if failureReason != "" {
		details["failureReason"] = failureReason
	}
	level := "info"
	msg := "Website health check completed"
	if status == "down" {
		level = "warn"
		msg = "Website health check reported downtime"
	}
	p.logger.Add(level, "worker", "website_checked", msg, &payload.WebsiteID, details)
	return nil
}

func (p *Processor) checkHTTP(ctx context.Context, target string) (string, int, int, string) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return "down", 0, 0, err.Error()
	}
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return "down", 0, latency, err.Error()
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return "up", resp.StatusCode, latency, ""
	}
	return "down", resp.StatusCode, latency, "non-successful status code"
}

func (p *Processor) checkKeyword(ctx context.Context, target string, keyword *string) (string, int, int, string) {
	if keyword == nil || *keyword == "" {
		return "down", 0, 0, "missing expected keyword"
	}
	reqCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return "down", 0, 0, err.Error()
	}
	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return "down", 0, latency, err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return "down", resp.StatusCode, latency, "non-successful status code"
	}
	if strings.Contains(string(body), *keyword) {
		return "up", resp.StatusCode, latency, ""
	}
	return "down", resp.StatusCode, latency, "expected keyword not found"
}

func (p *Processor) checkHeartbeat(website *models.Website) (string, int, int, string) {
	if website.LastHeartbeatAt == nil {
		return "down", 0, 0, "heartbeat never received"
	}
	grace := time.Duration(website.HeartbeatGraceSeconds) * time.Second
	if grace <= 0 {
		grace = 60 * time.Second
	}
	if time.Since(*website.LastHeartbeatAt) > grace {
		return "down", 0, 0, "heartbeat stale"
	}
	return "up", 200, 0, ""
}

func (p *Processor) checkTLSExpiry(target string, thresholdDays int) (string, int, int, string) {
	parsed, err := url.Parse(target)
	if err != nil || parsed.Host == "" {
		return "down", 0, 0, "invalid URL for TLS check"
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		host += ":443"
	}
	start := time.Now()
	conn, err := tls.DialWithDialer(&netDialer, "tcp", host, &tls.Config{ServerName: parsed.Hostname(), MinVersion: tls.VersionTLS12})
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

var netDialer = net.Dialer{Timeout: 5 * time.Second}
var alertHTTPClient = &http.Client{Timeout: 4 * time.Second}

func (p *Processor) handleIncidentsAndAlerts(ctx context.Context, websiteID int64, url string, status string, reason string, now time.Time) error {
	openIncident, err := p.repo.GetOpenIncident(ctx, websiteID)
	if err != nil {
		return err
	}

	muted, err := p.repo.IsWebsiteMuted(ctx, websiteID, now)
	if err != nil {
		return err
	}

	if status == "down" {
		if openIncident == nil {
			incidentID, createErr := p.repo.CreateIncident(ctx, websiteID, reason, now)
			if createErr != nil {
				return createErr
			}
			if !muted {
				p.dispatchAlerts(ctx, "incident_opened", websiteID, incidentID, url, reason)
			}
		}
		return nil
	}

	if openIncident != nil {
		if err = p.repo.ResolveIncident(ctx, openIncident.ID, now); err != nil {
			return err
		}
		if !muted {
			p.dispatchAlerts(ctx, "incident_resolved", websiteID, openIncident.ID, url, "service recovered")
		}
	}
	return nil
}

func (p *Processor) dispatchAlerts(ctx context.Context, event string, websiteID int64, incidentID int64, siteURL string, message string) {
	channels, err := p.repo.ListAlertChannels(ctx)
	if err != nil {
		p.logger.Add("error", "worker", "list_alert_channels_failed", "Failed to list alert channels", &websiteID, map[string]string{"error": err.Error()})
		return
	}

	payload := map[string]any{
		"event":      event,
		"websiteId":  websiteID,
		"incidentId": incidentID,
		"url":        siteURL,
		"message":    message,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)

	for _, channel := range channels {
		switch channel.ChannelType {
		case "webhook", "slack":
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, channel.Target, strings.NewReader(string(body)))
			if reqErr != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			resp, sendErr := alertHTTPClient.Do(req)
			if sendErr == nil && resp != nil {
				resp.Body.Close()
			}
		case "email":
			p.logger.Add("warn", "worker", "email_alert_not_configured", "Email channel exists but SMTP transport is not configured", &websiteID, map[string]string{"channel": channel.Name})
		}
	}
}
