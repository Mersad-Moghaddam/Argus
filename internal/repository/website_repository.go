package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"argus/internal/models"
)

// WebsiteRepository declares persistence operations for websites.
type WebsiteRepository interface {
	Create(ctx context.Context, website models.Website) (int64, error)
	List(ctx context.Context) ([]models.Website, error)
	Delete(ctx context.Context, id int64) error
	ListDue(ctx context.Context, now time.Time) ([]models.Website, error)
	MarkChecked(ctx context.Context, id int64, status string, statusCode int, latencyMS int, checkedAt, nextCheckAt time.Time) error
	RecordCheck(ctx context.Context, websiteID int64, status string, statusCode int, latencyMS int, failureReason string, checkedAt time.Time) error
	GetOpenIncident(ctx context.Context, websiteID int64) (*models.Incident, error)
	CreateIncident(ctx context.Context, websiteID int64, reason string, startedAt time.Time) (int64, error)
	ResolveIncident(ctx context.Context, incidentID int64, resolvedAt time.Time) error
	ListIncidents(ctx context.Context, websiteID *int64, state string, limit int) ([]models.Incident, error)
	ListAlertChannels(ctx context.Context) ([]models.AlertChannel, error)
	CreateAlertChannel(ctx context.Context, channel models.AlertChannel) (int64, error)
	CreateMaintenanceWindow(ctx context.Context, window models.MaintenanceWindow) (int64, error)
	IsWebsiteMuted(ctx context.Context, websiteID int64, now time.Time) (bool, error)
	CreateStatusPage(ctx context.Context, page models.StatusPage) (int64, error)
	ListStatusPages(ctx context.Context) ([]models.StatusPage, error)
	GetStatusPageBySlug(ctx context.Context, slug string) (*models.StatusPage, error)
	ListWebsitesByStatusPage(ctx context.Context, pageID int64) ([]models.Website, error)
	MarkHeartbeat(ctx context.Context, websiteID int64, checkedAt, nextCheckAt time.Time) error
}

// MySQLWebsiteRepository is a MySQL-backed website repository.
type MySQLWebsiteRepository struct {
	db *sql.DB
}

// NewMySQLWebsiteRepository creates a MySQLWebsiteRepository.
func NewMySQLWebsiteRepository(db *sql.DB) *MySQLWebsiteRepository {
	return &MySQLWebsiteRepository{db: db}
}

func scanWebsite(rows interface{ Scan(dest ...any) error }, w *models.Website) error {
	return rows.Scan(
		&w.ID,
		&w.URL,
		&w.HealthCheckURL,
		&w.CheckInterval,
		&w.MonitorType,
		&w.ExpectedKeyword,
		&w.TLSExpiryThresholdDays,
		&w.HeartbeatGraceSeconds,
		&w.Status,
		&w.LastCheckedAt,
		&w.NextCheckAt,
		&w.LastStatusCode,
		&w.LastLatencyMS,
		&w.StatusPageID,
		&w.CreatedAt,
		&w.UpdatedAt,
	)
}

// Create inserts a website row.
func (r *MySQLWebsiteRepository) Create(ctx context.Context, website models.Website) (int64, error) {
	const query = `
INSERT INTO websites (url, health_check_url, check_interval_seconds, monitor_type, expected_keyword, tls_expiry_threshold_days, heartbeat_grace_seconds, status, next_check_at, last_status_code, last_latency_ms, status_page_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query,
		website.URL,
		website.HealthCheckURL,
		website.CheckInterval,
		website.MonitorType,
		website.ExpectedKeyword,
		website.TLSExpiryThresholdDays,
		website.HeartbeatGraceSeconds,
		website.Status,
		website.NextCheckAt,
		website.LastStatusCode,
		website.LastLatencyMS,
		website.StatusPageID,
	)
	if err != nil {
		return 0, fmt.Errorf("insert website: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get website last insert id: %w", err)
	}
	return id, nil
}

// List returns all websites sorted by id desc.
func (r *MySQLWebsiteRepository) List(ctx context.Context) ([]models.Website, error) {
	const query = `
SELECT id, url, health_check_url, check_interval_seconds, monitor_type, expected_keyword, tls_expiry_threshold_days, heartbeat_grace_seconds, status, last_checked_at, next_check_at, last_status_code, last_latency_ms, status_page_id, created_at, updated_at
FROM websites
ORDER BY id DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query websites: %w", err)
	}
	defer rows.Close()

	items := make([]models.Website, 0)
	for rows.Next() {
		var w models.Website
		if err = scanWebsite(rows, &w); err != nil {
			return nil, fmt.Errorf("scan website row: %w", err)
		}
		items = append(items, w)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate website rows: %w", err)
	}
	return items, nil
}

// Delete removes a website by id.
func (r *MySQLWebsiteRepository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM websites WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete website: %w", err)
	}
	return nil
}

// ListDue returns websites that should be checked now.
func (r *MySQLWebsiteRepository) ListDue(ctx context.Context, now time.Time) ([]models.Website, error) {
	const query = `
SELECT id, url, health_check_url, check_interval_seconds, monitor_type, expected_keyword, tls_expiry_threshold_days, heartbeat_grace_seconds, status, last_checked_at, next_check_at, last_status_code, last_latency_ms, status_page_id, created_at, updated_at
FROM websites
WHERE next_check_at <= ?
ORDER BY next_check_at ASC`

	rows, err := r.db.QueryContext(ctx, query, now)
	if err != nil {
		return nil, fmt.Errorf("query due websites: %w", err)
	}
	defer rows.Close()

	items := make([]models.Website, 0)
	for rows.Next() {
		var w models.Website
		if err = scanWebsite(rows, &w); err != nil {
			return nil, fmt.Errorf("scan due website row: %w", err)
		}
		items = append(items, w)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due website rows: %w", err)
	}
	return items, nil
}

// MarkChecked updates status, status code, latency and next check time after execution.
func (r *MySQLWebsiteRepository) MarkChecked(ctx context.Context, id int64, status string, statusCode int, latencyMS int, checkedAt, nextCheckAt time.Time) error {
	const query = `
UPDATE websites
SET status = ?, last_status_code = ?, last_latency_ms = ?, last_checked_at = ?, next_check_at = ?, updated_at = NOW()
WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, status, statusCode, latencyMS, checkedAt, nextCheckAt, id)
	if err != nil {
		return fmt.Errorf("update checked website: %w", err)
	}
	return nil
}

func (r *MySQLWebsiteRepository) RecordCheck(ctx context.Context, websiteID int64, status string, statusCode int, latencyMS int, failureReason string, checkedAt time.Time) error {
	const query = `INSERT INTO website_checks (website_id, status, status_code, latency_ms, failure_reason, checked_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, websiteID, status, statusCode, latencyMS, nullableString(failureReason), checkedAt)
	if err != nil {
		return fmt.Errorf("insert website check: %w", err)
	}
	return nil
}

func (r *MySQLWebsiteRepository) GetOpenIncident(ctx context.Context, websiteID int64) (*models.Incident, error) {
	const query = `SELECT id, website_id, state, started_at, acknowledged_at, resolved_at, last_failure_reason, created_at, updated_at FROM incidents WHERE website_id = ? AND state IN ('open','acknowledged') ORDER BY started_at DESC LIMIT 1`
	var item models.Incident
	var reason sql.NullString
	if err := r.db.QueryRowContext(ctx, query, websiteID).Scan(&item.ID, &item.WebsiteID, &item.State, &item.StartedAt, &item.AcknowledgedAt, &item.ResolvedAt, &reason, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query open incident: %w", err)
	}
	if reason.Valid {
		item.LastFailureReason = &reason.String
	}
	return &item, nil
}

func (r *MySQLWebsiteRepository) CreateIncident(ctx context.Context, websiteID int64, reason string, startedAt time.Time) (int64, error) {
	const query = `INSERT INTO incidents (website_id, state, started_at, last_failure_reason) VALUES (?, 'open', ?, ?)`
	result, err := r.db.ExecContext(ctx, query, websiteID, startedAt, nullableString(reason))
	if err != nil {
		return 0, fmt.Errorf("insert incident: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("incident last insert id: %w", err)
	}
	return id, nil
}

func (r *MySQLWebsiteRepository) ResolveIncident(ctx context.Context, incidentID int64, resolvedAt time.Time) error {
	const query = `UPDATE incidents SET state='resolved', resolved_at=?, updated_at=NOW() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, resolvedAt, incidentID)
	if err != nil {
		return fmt.Errorf("resolve incident: %w", err)
	}
	return nil
}

func (r *MySQLWebsiteRepository) ListIncidents(ctx context.Context, websiteID *int64, state string, limit int) ([]models.Incident, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `SELECT id, website_id, state, started_at, acknowledged_at, resolved_at, last_failure_reason, created_at, updated_at FROM incidents`
	args := make([]any, 0, 3)
	where := ""
	if websiteID != nil {
		where = "website_id = ?"
		args = append(args, *websiteID)
	}
	if state != "" {
		if where != "" {
			where += " AND "
		}
		where += "state = ?"
		args = append(args, state)
	}
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY started_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list incidents: %w", err)
	}
	defer rows.Close()

	items := make([]models.Incident, 0)
	for rows.Next() {
		var item models.Incident
		var reason sql.NullString
		if err = rows.Scan(&item.ID, &item.WebsiteID, &item.State, &item.StartedAt, &item.AcknowledgedAt, &item.ResolvedAt, &reason, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan incident row: %w", err)
		}
		if reason.Valid {
			item.LastFailureReason = &reason.String
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incidents: %w", err)
	}
	return items, nil
}

func (r *MySQLWebsiteRepository) ListAlertChannels(ctx context.Context) ([]models.AlertChannel, error) {
	const query = `SELECT id, name, channel_type, target, enabled, created_at, updated_at FROM alert_channels WHERE enabled = 1 ORDER BY id DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list alert channels: %w", err)
	}
	defer rows.Close()

	items := make([]models.AlertChannel, 0)
	for rows.Next() {
		var c models.AlertChannel
		if err = rows.Scan(&c.ID, &c.Name, &c.ChannelType, &c.Target, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan alert channel: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (r *MySQLWebsiteRepository) CreateAlertChannel(ctx context.Context, channel models.AlertChannel) (int64, error) {
	const query = `INSERT INTO alert_channels (name, channel_type, target, enabled) VALUES (?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, channel.Name, channel.ChannelType, channel.Target, channel.Enabled)
	if err != nil {
		return 0, fmt.Errorf("insert alert channel: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("alert channel id: %w", err)
	}
	return id, nil
}

func (r *MySQLWebsiteRepository) CreateMaintenanceWindow(ctx context.Context, window models.MaintenanceWindow) (int64, error) {
	const query = `INSERT INTO maintenance_windows (website_id, starts_at, ends_at, mute_alerts, reason) VALUES (?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, window.WebsiteID, window.StartsAt, window.EndsAt, window.MuteAlerts, window.Reason)
	if err != nil {
		return 0, fmt.Errorf("insert maintenance window: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("maintenance window id: %w", err)
	}
	return id, nil
}

func (r *MySQLWebsiteRepository) IsWebsiteMuted(ctx context.Context, websiteID int64, now time.Time) (bool, error) {
	const query = `SELECT COUNT(1) FROM maintenance_windows WHERE mute_alerts = 1 AND starts_at <= ? AND ends_at >= ? AND (website_id IS NULL OR website_id = ?)`
	var count int
	if err := r.db.QueryRowContext(ctx, query, now, now, websiteID).Scan(&count); err != nil {
		return false, fmt.Errorf("query maintenance windows: %w", err)
	}
	return count > 0, nil
}

func (r *MySQLWebsiteRepository) CreateStatusPage(ctx context.Context, page models.StatusPage) (int64, error) {
	const query = `INSERT INTO status_pages (slug, title, is_public) VALUES (?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, page.Slug, page.Title, page.IsPublic)
	if err != nil {
		return 0, fmt.Errorf("insert status page: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("status page id: %w", err)
	}
	return id, nil
}

func (r *MySQLWebsiteRepository) ListStatusPages(ctx context.Context) ([]models.StatusPage, error) {
	const query = `SELECT id, slug, title, is_public, created_at, updated_at FROM status_pages ORDER BY id DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list status pages: %w", err)
	}
	defer rows.Close()

	items := make([]models.StatusPage, 0)
	for rows.Next() {
		var p models.StatusPage
		if err = rows.Scan(&p.ID, &p.Slug, &p.Title, &p.IsPublic, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan status page: %w", err)
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func (r *MySQLWebsiteRepository) GetStatusPageBySlug(ctx context.Context, slug string) (*models.StatusPage, error) {
	const query = `SELECT id, slug, title, is_public, created_at, updated_at FROM status_pages WHERE slug = ? LIMIT 1`
	var p models.StatusPage
	if err := r.db.QueryRowContext(ctx, query, slug).Scan(&p.ID, &p.Slug, &p.Title, &p.IsPublic, &p.CreatedAt, &p.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get status page by slug: %w", err)
	}
	return &p, nil
}

func (r *MySQLWebsiteRepository) ListWebsitesByStatusPage(ctx context.Context, pageID int64) ([]models.Website, error) {
	const query = `SELECT id, url, health_check_url, check_interval_seconds, monitor_type, expected_keyword, tls_expiry_threshold_days, heartbeat_grace_seconds, status, last_checked_at, next_check_at, last_status_code, last_latency_ms, status_page_id, created_at, updated_at FROM websites WHERE status_page_id = ? ORDER BY id DESC`
	rows, err := r.db.QueryContext(ctx, query, pageID)
	if err != nil {
		return nil, fmt.Errorf("list websites by status page: %w", err)
	}
	defer rows.Close()
	items := make([]models.Website, 0)
	for rows.Next() {
		var w models.Website
		if err = scanWebsite(rows, &w); err != nil {
			return nil, fmt.Errorf("scan status page website: %w", err)
		}
		items = append(items, w)
	}
	return items, rows.Err()
}

func (r *MySQLWebsiteRepository) MarkHeartbeat(ctx context.Context, websiteID int64, checkedAt, nextCheckAt time.Time) error {
	const query = `UPDATE websites SET status='up', last_status_code=200, last_latency_ms=0, last_checked_at=?, next_check_at=?, updated_at=NOW() WHERE id=? AND monitor_type='heartbeat'`
	_, err := r.db.ExecContext(ctx, query, checkedAt, nextCheckAt, websiteID)
	if err != nil {
		return fmt.Errorf("mark heartbeat: %w", err)
	}
	return nil
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
