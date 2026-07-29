package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"

	"argus/internal/agent"
	"argus/internal/domain"
	"argus/internal/domain/ports"
	"argus/internal/models"
	"argus/internal/observability"
)

var ErrHeartbeatNotFound = errors.New("heartbeat monitor not found")

type Service struct {
	monitors    ports.MonitorStore
	incidents   ports.IncidentStore
	maintenance ports.MaintenanceStore
	statusPages ports.StatusPageStore
	alerts      ports.AlertChannelStore
	outbox      ports.OutboxStore
	logger      *observability.LogStore

	// Project-based API route monitoring dependencies.
	users                ports.UserStore
	tokens               ports.AuthTokenStore
	passwordRecovery     ports.PasswordRecoveryStore
	recoveryDelivery     ports.RecoveryDelivery
	projects             ports.ProjectStore
	routes               ports.RouteStore
	routeIncidents       ports.RouteIncidentStore
	imports              ports.ImportStore
	telemetryCredentials ports.TelemetryCredentialStore
	telemetryIngress     ports.TelemetryIngressStore
	telemetryMappings    ports.TelemetryMappingStore
	slos                 ports.SLOStore
	heartbeats           ports.HeartbeatStore
	privateAgents        ports.PrivateAgentStore
	privateAgentResults  ports.PrivateAgentResultStore
	agentConfigSigner    *agent.ConfigurationSigner
	projectIncidents     ports.ProjectIncidentStore
}

func NewService(monitors ports.MonitorStore, incidents ports.IncidentStore, maintenance ports.MaintenanceStore, statusPages ports.StatusPageStore, alerts ports.AlertChannelStore, outbox ports.OutboxStore, logger *observability.LogStore,
	users ports.UserStore, tokens ports.AuthTokenStore, passwordRecovery ports.PasswordRecoveryStore, recoveryDelivery ports.RecoveryDelivery, projects ports.ProjectStore, routes ports.RouteStore, routeIncidents ports.RouteIncidentStore, imports ports.ImportStore, telemetryCredentials ports.TelemetryCredentialStore, telemetryIngress ports.TelemetryIngressStore, telemetryMappings ports.TelemetryMappingStore, slos ports.SLOStore, heartbeats ports.HeartbeatStore) *Service {
	return &Service{monitors: monitors, incidents: incidents, maintenance: maintenance, statusPages: statusPages, alerts: alerts, outbox: outbox, logger: logger,
		users: users, tokens: tokens, passwordRecovery: passwordRecovery, recoveryDelivery: recoveryDelivery, projects: projects, routes: routes, routeIncidents: routeIncidents, imports: imports, telemetryCredentials: telemetryCredentials, telemetryIngress: telemetryIngress, telemetryMappings: telemetryMappings, slos: slos, heartbeats: heartbeats}
}

func (s *Service) SetPrivateAgentStore(store ports.PrivateAgentStore) { s.privateAgents = store }
func (s *Service) SetPrivateAgentResultStore(store ports.PrivateAgentResultStore) {
	s.privateAgentResults = store
}

// SetProjectIncidentStore attaches the source-agnostic incident lifecycle.
// It is optional during the additive migration so older callers remain valid.
func (s *Service) SetProjectIncidentStore(store ports.ProjectIncidentStore) {
	s.projectIncidents = store
}

type CreateMonitorInput struct {
	URL                    string
	HealthCheckURL         *string
	CheckInterval          int
	MonitorType            string
	ExpectedKeyword        *string
	TLSExpiryThresholdDays int
	HeartbeatGraceSeconds  int
	StatusPageID           *int64
}

func (s *Service) CreateMonitor(ctx context.Context, input CreateMonitorInput) (models.Website, error) {
	normalized, err := domain.NormalizeMonitor(domain.Monitor{
		URL:                    input.URL,
		HealthCheckURL:         input.HealthCheckURL,
		CheckIntervalSeconds:   input.CheckInterval,
		MonitorType:            input.MonitorType,
		ExpectedKeyword:        input.ExpectedKeyword,
		TLSExpiryThresholdDays: input.TLSExpiryThresholdDays,
		HeartbeatGraceSeconds:  input.HeartbeatGraceSeconds,
		StatusPageID:           input.StatusPageID,
	})
	if err != nil {
		return models.Website{}, err
	}

	website := models.Website{URL: normalized.URL, HealthCheckURL: normalized.HealthCheckURL, CheckInterval: normalized.CheckIntervalSeconds, MonitorType: normalized.MonitorType, ExpectedKeyword: normalized.ExpectedKeyword, TLSExpiryThresholdDays: normalized.TLSExpiryThresholdDays, HeartbeatGraceSeconds: normalized.HeartbeatGraceSeconds, Status: normalized.Status, NextCheckAt: normalized.NextCheckAt, StatusPageID: normalized.StatusPageID}
	id, err := s.monitors.Create(ctx, website)
	if err != nil {
		return models.Website{}, err
	}
	website.ID = id
	s.logger.Add("info", "api", "monitor_created", "Monitor created", &id, map[string]string{"url": website.URL, "monitorType": website.MonitorType, "interval": strconv.Itoa(website.CheckInterval)})
	return website, nil
}

func (s *Service) ListMonitors(ctx context.Context, limit, offset int) ([]models.Website, error) {
	return s.monitors.List(ctx, limit, offset)
}
func (s *Service) ListChecks(ctx context.Context, websiteID *int64, limit int) ([]models.WebsiteCheck, error) {
	return s.monitors.ListChecks(ctx, websiteID, limit)
}
func (s *Service) DeleteMonitor(ctx context.Context, id int64) error {
	return s.monitors.Delete(ctx, id)
}
func (s *Service) ListIncidents(ctx context.Context, websiteID *int64, state string, limit, offset int) ([]models.Incident, error) {
	return s.incidents.ListIncidents(ctx, websiteID, state, limit, offset)
}
func (s *Service) CreateAlertChannel(ctx context.Context, channel models.AlertChannel) (int64, error) {
	channel.Name = strings.TrimSpace(channel.Name)
	channel.ChannelType = strings.ToLower(strings.TrimSpace(channel.ChannelType))
	channel.Target = strings.TrimSpace(channel.Target)
	if channel.Name == "" || len(channel.Name) > 120 || (channel.ChannelType != "webhook" && channel.ChannelType != "slack") {
		return 0, errors.New("invalid alert channel")
	}
	target, err := url.Parse(channel.Target)
	if err != nil || target.Scheme != "https" || target.Host == "" || target.User != nil || target.Fragment != "" {
		return 0, errors.New("alert channel target must be an absolute HTTPS URL without userinfo or fragment")
	}
	host := strings.ToLower(strings.TrimSuffix(target.Hostname(), "."))
	if host == "localhost" || host == "metadata.google.internal" || host == "metadata.goog" || host == "169.254.169.254" {
		return 0, errors.New("alert channel target must not be a local or metadata endpoint")
	}
	if address, parseErr := netip.ParseAddr(host); parseErr == nil && (address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() || address.IsUnspecified()) {
		return 0, errors.New("alert channel target must not be a local or private address")
	}
	return s.alerts.CreateAlertChannel(ctx, channel)
}
func (s *Service) CreateMaintenanceWindow(ctx context.Context, window models.MaintenanceWindow) (int64, error) {
	return s.maintenance.CreateMaintenanceWindow(ctx, window)
}
func (s *Service) CreateStatusPage(ctx context.Context, page models.StatusPage) (int64, error) {
	return s.statusPages.CreateStatusPage(ctx, page)
}
func (s *Service) ListStatusPages(ctx context.Context, limit, offset int) ([]models.StatusPage, error) {
	return s.statusPages.ListStatusPages(ctx, limit, offset)
}
func (s *Service) GetStatusPage(ctx context.Context, slug string) (*models.StatusPage, []models.Website, error) {
	page, err := s.statusPages.GetStatusPageBySlug(ctx, slug)
	if err != nil || page == nil {
		return page, nil, err
	}
	websites, err := s.statusPages.ListWebsitesByStatusPage(ctx, page.ID)
	return page, websites, err
}
func (s *Service) MarkHeartbeat(ctx context.Context, websiteID int64) error {
	now := time.Now().UTC()
	if err := s.monitors.MarkHeartbeat(ctx, websiteID, now, now.Add(30*time.Second)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrHeartbeatNotFound
		}
		return err
	}
	return nil
}

type OutboxPayload struct {
	Event      string `json:"event"`
	WebsiteID  int64  `json:"websiteId"`
	IncidentID int64  `json:"incidentId"`
	URL        string `json:"url"`
	Message    string `json:"message"`
	Timestamp  string `json:"timestamp"`
}

func (s *Service) ProcessIncidentTransition(ctx context.Context, websiteID int64, url, status, reason string, now time.Time) error {
	openIncident, err := s.incidents.GetOpenIncident(ctx, websiteID)
	if err != nil {
		return err
	}
	transition := domain.IncidentPolicy(openIncident != nil, status)
	if !transition.ShouldOpen && !transition.ShouldResolve {
		return nil
	}

	muted, err := s.maintenance.IsWebsiteMuted(ctx, websiteID, now)
	if err != nil {
		return err
	}
	if domain.ShouldSuppressAlerts(muted) {
		return nil
	}

	bucket := now.UTC().Truncate(time.Minute).Format(time.RFC3339)
	if transition.ShouldOpen {
		incidentID, createErr := s.incidents.CreateIncident(ctx, websiteID, reason, now)
		if createErr != nil {
			return createErr
		}
		payload, _ := json.Marshal(OutboxPayload{Event: "incident_opened", WebsiteID: websiteID, IncidentID: incidentID, URL: url, Message: reason, Timestamp: now.Format(time.RFC3339)})
		dedupe := fmt.Sprintf("%d:incident_opened:%s", websiteID, bucket)
		return s.outbox.AddEvent(ctx, "incident_opened", incidentID, dedupe, payload, now)
	}

	if openIncident == nil {
		return nil
	}
	if err = s.incidents.ResolveIncident(ctx, openIncident.ID, now); err != nil {
		return err
	}
	payload, _ := json.Marshal(OutboxPayload{Event: "incident_resolved", WebsiteID: websiteID, IncidentID: openIncident.ID, URL: url, Message: "service recovered", Timestamp: now.Format(time.RFC3339)})
	dedupe := fmt.Sprintf("%d:incident_resolved:%s", websiteID, bucket)
	return s.outbox.AddEvent(ctx, "incident_resolved", openIncident.ID, dedupe, payload, now)
}
