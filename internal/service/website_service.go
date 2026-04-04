package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"argus/internal/models"
	"argus/internal/observability"
	"argus/internal/repository"
)

var (
	ErrInvalidURL         = errors.New("invalid URL")
	ErrInvalidInterval    = errors.New("checkInterval must be at least 10 seconds")
	ErrInvalidMonitorType = errors.New("invalid monitorType")
)

// WebsiteService orchestrates website business logic.
type WebsiteService struct {
	repo   repository.WebsiteRepository
	logger *observability.LogStore
}

// CreateWebsiteInput contains all create-time settings.
type CreateWebsiteInput struct {
	URL                    string
	HealthCheckURL         *string
	CheckInterval          int
	MonitorType            string
	ExpectedKeyword        *string
	TLSExpiryThresholdDays int
	HeartbeatGraceSeconds  int
	StatusPageID           *int64
}

func NewWebsiteService(repo repository.WebsiteRepository, logger *observability.LogStore) *WebsiteService {
	return &WebsiteService{repo: repo, logger: logger}
}

func (s *WebsiteService) CreateWebsite(ctx context.Context, input CreateWebsiteInput) (models.Website, error) {
	if input.CheckInterval < 10 {
		return models.Website{}, ErrInvalidInterval
	}

	monitorType := input.MonitorType
	if monitorType == "" {
		monitorType = models.MonitorTypeHTTPStatus
	}
	if !isValidMonitorType(monitorType) {
		return models.Website{}, ErrInvalidMonitorType
	}

	baseURL, err := parseHTTPURL(input.URL)
	if err != nil {
		return models.Website{}, err
	}

	var normalizedHealthURL *string
	if input.HealthCheckURL != nil && *input.HealthCheckURL != "" {
		normalized, healthErr := parseHTTPURL(*input.HealthCheckURL)
		if healthErr != nil {
			return models.Website{}, healthErr
		}
		normalizedHealthURL = &normalized
	}

	if monitorType == models.MonitorTypeKeyword && (input.ExpectedKeyword == nil || strings.TrimSpace(*input.ExpectedKeyword) == "") {
		return models.Website{}, errors.New("expectedKeyword is required for keyword monitor")
	}

	if monitorType == models.MonitorTypeTLSExpiry && input.TLSExpiryThresholdDays <= 0 {
		input.TLSExpiryThresholdDays = 14
	}
	if monitorType == models.MonitorTypeHeartbeat && input.HeartbeatGraceSeconds < 10 {
		input.HeartbeatGraceSeconds = input.CheckInterval * 2
	}

	now := time.Now().UTC()
	website := models.Website{
		URL:                    baseURL,
		HealthCheckURL:         normalizedHealthURL,
		CheckInterval:          input.CheckInterval,
		MonitorType:            monitorType,
		ExpectedKeyword:        input.ExpectedKeyword,
		TLSExpiryThresholdDays: input.TLSExpiryThresholdDays,
		HeartbeatGraceSeconds:  input.HeartbeatGraceSeconds,
		Status:                 "pending",
		NextCheckAt:            now,
		LastStatusCode:         0,
		LastLatencyMS:          0,
		StatusPageID:           input.StatusPageID,
	}

	id, err := s.repo.Create(ctx, website)
	if err != nil {
		return models.Website{}, fmt.Errorf("create website in repository: %w", err)
	}
	website.ID = id
	website.CreatedAt = now
	website.UpdatedAt = now

	details := map[string]string{
		"url":             website.URL,
		"checkInterval":   strconv.Itoa(website.CheckInterval),
		"monitorType":     website.MonitorType,
		"initialStatus":   website.Status,
		"nextScheduledAt": website.NextCheckAt.Format(time.RFC3339),
	}
	if website.HealthCheckURL != nil {
		details["healthCheckUrl"] = *website.HealthCheckURL
	}
	s.logger.Add("info", "api", "website_created", "Website was added for monitoring", &website.ID, details)

	return website, nil
}

func (s *WebsiteService) ListWebsites(ctx context.Context) ([]models.Website, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list websites in repository: %w", err)
	}
	return items, nil
}

func (s *WebsiteService) DeleteWebsite(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete website in repository: %w", err)
	}
	s.logger.Add("warn", "api", "website_deleted", "Website was removed from monitoring", &id, nil)
	return nil
}

func (s *WebsiteService) ListIncidents(ctx context.Context, websiteID *int64, state string, limit int) ([]models.Incident, error) {
	return s.repo.ListIncidents(ctx, websiteID, state, limit)
}

func (s *WebsiteService) CreateAlertChannel(ctx context.Context, channel models.AlertChannel) (int64, error) {
	if channel.Name == "" || channel.Target == "" {
		return 0, errors.New("name and target are required")
	}
	if channel.ChannelType == "" {
		channel.ChannelType = "webhook"
	}
	channel.Enabled = true
	return s.repo.CreateAlertChannel(ctx, channel)
}

func (s *WebsiteService) CreateMaintenanceWindow(ctx context.Context, window models.MaintenanceWindow) (int64, error) {
	if !window.EndsAt.After(window.StartsAt) {
		return 0, errors.New("endsAt must be after startsAt")
	}
	window.MuteAlerts = true
	return s.repo.CreateMaintenanceWindow(ctx, window)
}

func (s *WebsiteService) CreateStatusPage(ctx context.Context, page models.StatusPage) (int64, error) {
	if page.Slug == "" || page.Title == "" {
		return 0, errors.New("slug and title are required")
	}
	page.IsPublic = true
	return s.repo.CreateStatusPage(ctx, page)
}

func (s *WebsiteService) ListStatusPages(ctx context.Context) ([]models.StatusPage, error) {
	return s.repo.ListStatusPages(ctx)
}

func (s *WebsiteService) GetStatusPage(ctx context.Context, slug string) (*models.StatusPage, []models.Website, error) {
	page, err := s.repo.GetStatusPageBySlug(ctx, slug)
	if err != nil || page == nil {
		return page, nil, err
	}
	websites, err := s.repo.ListWebsitesByStatusPage(ctx, page.ID)
	if err != nil {
		return nil, nil, err
	}
	return page, websites, nil
}

func (s *WebsiteService) MarkHeartbeat(ctx context.Context, websiteID int64) error {
	now := time.Now().UTC()
	next := now.Add(30 * time.Second)
	return s.repo.MarkHeartbeat(ctx, websiteID, now, next)
}

func parseHTTPURL(raw string) (string, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", ErrInvalidURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidURL
	}
	return parsed.String(), nil
}

func isValidMonitorType(value string) bool {
	switch value {
	case models.MonitorTypeHTTPStatus, models.MonitorTypeKeyword, models.MonitorTypeHeartbeat, models.MonitorTypeTLSExpiry:
		return true
	default:
		return false
	}
}
