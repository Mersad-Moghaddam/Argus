package ports

import (
	"context"
	"time"

	"argus/internal/models"
)

type MonitorStore interface {
	Create(ctx context.Context, website models.Website) (int64, error)
	GetByID(ctx context.Context, id int64) (*models.Website, error)
	List(ctx context.Context, limit, offset int) ([]models.Website, error)
	Delete(ctx context.Context, id int64) error
	ListDue(ctx context.Context, now time.Time, limit int, afterID int64) ([]models.Website, error)
	MarkChecked(ctx context.Context, id int64, status string, statusCode int, latencyMS int, checkedAt, nextCheckAt time.Time) error
	RecordCheck(ctx context.Context, websiteID int64, status string, statusCode int, latencyMS int, failureReason string, checkedAt time.Time) error
	MarkHeartbeat(ctx context.Context, websiteID int64, checkedAt, nextCheckAt time.Time) error
}

type IncidentStore interface {
	GetOpenIncident(ctx context.Context, websiteID int64) (*models.Incident, error)
	CreateIncident(ctx context.Context, websiteID int64, reason string, startedAt time.Time) (int64, error)
	ResolveIncident(ctx context.Context, incidentID int64, resolvedAt time.Time) error
	ListIncidents(ctx context.Context, websiteID *int64, state string, limit int, offset int) ([]models.Incident, error)
}

type MaintenanceStore interface {
	CreateMaintenanceWindow(ctx context.Context, window models.MaintenanceWindow) (int64, error)
	IsWebsiteMuted(ctx context.Context, websiteID int64, now time.Time) (bool, error)
}

type StatusPageStore interface {
	CreateStatusPage(ctx context.Context, page models.StatusPage) (int64, error)
	ListStatusPages(ctx context.Context, limit, offset int) ([]models.StatusPage, error)
	GetStatusPageBySlug(ctx context.Context, slug string) (*models.StatusPage, error)
	ListWebsitesByStatusPage(ctx context.Context, pageID int64) ([]models.Website, error)
}

type AlertChannelStore interface {
	ListAlertChannels(ctx context.Context) ([]models.AlertChannel, error)
	CreateAlertChannel(ctx context.Context, channel models.AlertChannel) (int64, error)
}

type OutboxStore interface {
	AddEvent(ctx context.Context, eventType string, aggregateID int64, dedupeKey string, payload []byte, availableAt time.Time) error
	FetchPending(ctx context.Context, limit int) ([]models.OutboxEvent, error)
	MarkProcessed(ctx context.Context, eventID int64) error
	MarkFailed(ctx context.Context, eventID int64, message string) error
}

type Notifier interface {
	Notify(ctx context.Context, channels []models.AlertChannel, payload []byte) error
}

type Clock interface{ Now() time.Time }
