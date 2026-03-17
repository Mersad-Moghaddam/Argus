package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"argus/internal/models"
	"argus/internal/observability"
	"argus/internal/repository"
)

var (
	// ErrInvalidURL is returned when a website URL is malformed.
	ErrInvalidURL = errors.New("invalid URL")
	// ErrInvalidInterval is returned when check interval is too small.
	ErrInvalidInterval = errors.New("checkInterval must be at least 10 seconds")
)

// WebsiteService orchestrates website business logic.
type WebsiteService struct {
	repo   repository.WebsiteRepository
	logger *observability.LogStore
}

// NewWebsiteService creates a WebsiteService.
func NewWebsiteService(repo repository.WebsiteRepository, logger *observability.LogStore) *WebsiteService {
	return &WebsiteService{repo: repo, logger: logger}
}

// CreateWebsite validates and stores a monitored website.
func (s *WebsiteService) CreateWebsite(ctx context.Context, rawURL string, interval int) (models.Website, error) {
	if interval < 10 {
		return models.Website{}, ErrInvalidInterval
	}
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return models.Website{}, ErrInvalidURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return models.Website{}, ErrInvalidURL
	}

	now := time.Now().UTC()
	website := models.Website{
		URL:            parsed.String(),
		CheckInterval:  interval,
		Status:         "pending",
		NextCheckAt:    now,
		LastStatusCode: 0,
	}

	id, err := s.repo.Create(ctx, website)
	if err != nil {
		return models.Website{}, fmt.Errorf("create website in repository: %w", err)
	}
	website.ID = id
	website.CreatedAt = now
	website.UpdatedAt = now

	s.logger.Add("info", "api", "website_created", "Website was added for monitoring", &website.ID, map[string]string{
		"url":             website.URL,
		"checkInterval":   strconv.Itoa(website.CheckInterval),
		"initialStatus":   website.Status,
		"nextScheduledAt": website.NextCheckAt.Format(time.RFC3339),
	})

	return website, nil
}

// ListWebsites returns all monitored websites.
func (s *WebsiteService) ListWebsites(ctx context.Context) ([]models.Website, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list websites in repository: %w", err)
	}
	return items, nil
}

// DeleteWebsite removes a website entry.
func (s *WebsiteService) DeleteWebsite(ctx context.Context, id int64) error {
	err := s.repo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("delete website in repository: %w", err)
	}

	s.logger.Add("warn", "api", "website_deleted", "Website was removed from monitoring", &id, nil)
	return nil
}
