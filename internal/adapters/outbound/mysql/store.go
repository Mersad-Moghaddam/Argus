package mysql

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"argus/internal/models"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func scanWebsite(rows interface{ Scan(dest ...any) error }, w *models.Website) error {
	return rows.Scan(&w.ID, &w.URL, &w.HealthCheckURL, &w.CheckInterval, &w.MonitorType, &w.ExpectedKeyword, &w.TLSExpiryThresholdDays, &w.HeartbeatGraceSeconds, &w.Status, &w.LastCheckedAt, &w.LastHeartbeatAt, &w.NextCheckAt, &w.LastStatusCode, &w.LastLatencyMS, &w.StatusPageID, &w.CreatedAt, &w.UpdatedAt)
}

func (r *Store) Create(ctx context.Context, website models.Website) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO websites (url, health_check_url, check_interval_seconds, monitor_type, expected_keyword, tls_expiry_threshold_days, heartbeat_grace_seconds, status, next_check_at, last_status_code, last_latency_ms, status_page_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, website.URL, website.HealthCheckURL, website.CheckInterval, website.MonitorType, website.ExpectedKeyword, website.TLSExpiryThresholdDays, website.HeartbeatGraceSeconds, website.Status, website.NextCheckAt, website.LastStatusCode, website.LastLatencyMS, website.StatusPageID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (r *Store) GetByID(ctx context.Context, id int64) (*models.Website, error) {
	var w models.Website
	err := r.db.QueryRowContext(ctx, `SELECT id, url, health_check_url, check_interval_seconds, monitor_type, expected_keyword, tls_expiry_threshold_days, heartbeat_grace_seconds, status, last_checked_at, last_heartbeat_received_at, next_check_at, last_status_code, last_latency_ms, status_page_id, created_at, updated_at FROM websites WHERE id=? LIMIT 1`, id).Scan(&w.ID, &w.URL, &w.HealthCheckURL, &w.CheckInterval, &w.MonitorType, &w.ExpectedKeyword, &w.TLSExpiryThresholdDays, &w.HeartbeatGraceSeconds, &w.Status, &w.LastCheckedAt, &w.LastHeartbeatAt, &w.NextCheckAt, &w.LastStatusCode, &w.LastLatencyMS, &w.StatusPageID, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &w, err
}
func (r *Store) List(ctx context.Context, limit, offset int) ([]models.Website, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, url, health_check_url, check_interval_seconds, monitor_type, expected_keyword, tls_expiry_threshold_days, heartbeat_grace_seconds, status, last_checked_at, last_heartbeat_received_at, next_check_at, last_status_code, last_latency_ms, status_page_id, created_at, updated_at FROM websites ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Website{}
	for rows.Next() {
		var w models.Website
		if err = scanWebsite(rows, &w); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func (r *Store) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM websites WHERE id=?`, id)
	return err
}
func (r *Store) ListDue(ctx context.Context, now time.Time, limit int, afterID int64) ([]models.Website, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, url, health_check_url, check_interval_seconds, monitor_type, expected_keyword, tls_expiry_threshold_days, heartbeat_grace_seconds, status, last_checked_at, last_heartbeat_received_at, next_check_at, last_status_code, last_latency_ms, status_page_id, created_at, updated_at FROM websites WHERE next_check_at <= ? AND id > ? ORDER BY id ASC LIMIT ?`, now, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Website{}
	for rows.Next() {
		var w models.Website
		if err = scanWebsite(rows, &w); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
func (r *Store) MarkChecked(ctx context.Context, id int64, status string, statusCode int, latencyMS int, checkedAt, nextCheckAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE websites SET status=?, last_status_code=?, last_latency_ms=?, last_checked_at=?, next_check_at=?, updated_at=NOW() WHERE id=?`, status, statusCode, latencyMS, checkedAt, nextCheckAt, id)
	return err
}
func (r *Store) RecordCheck(ctx context.Context, websiteID int64, status string, statusCode int, latencyMS int, failureReason string, checkedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO website_checks (website_id,status,status_code,latency_ms,failure_reason,checked_at) VALUES (?, ?, ?, ?, ?, ?)`, websiteID, status, statusCode, latencyMS, nullableString(failureReason), checkedAt)
	return err
}
func (r *Store) MarkHeartbeat(ctx context.Context, websiteID int64, checkedAt, nextCheckAt time.Time) error {
	res, err := r.db.ExecContext(ctx, `UPDATE websites SET status='up', last_status_code=200, last_latency_ms=0, last_checked_at=?, last_heartbeat_received_at=?, next_check_at=?, updated_at=NOW() WHERE id=? AND monitor_type='heartbeat'`, checkedAt, checkedAt, nextCheckAt, websiteID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
func (r *Store) GetOpenIncident(ctx context.Context, websiteID int64) (*models.Incident, error) {
	var i models.Incident
	var reason sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, website_id, state, started_at, acknowledged_at, resolved_at, last_failure_reason, created_at, updated_at FROM incidents WHERE website_id=? AND state IN ('open','acknowledged') ORDER BY started_at DESC LIMIT 1`, websiteID).Scan(&i.ID, &i.WebsiteID, &i.State, &i.StartedAt, &i.AcknowledgedAt, &i.ResolvedAt, &reason, &i.CreatedAt, &i.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if reason.Valid {
		i.LastFailureReason = &reason.String
	}
	return &i, nil
}
func (r *Store) CreateIncident(ctx context.Context, websiteID int64, reason string, startedAt time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO incidents (website_id,state,started_at,last_failure_reason) VALUES (?, 'open', ?, ?)`, websiteID, startedAt, nullableString(reason))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (r *Store) ResolveIncident(ctx context.Context, incidentID int64, resolvedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE incidents SET state='resolved', resolved_at=?, updated_at=NOW() WHERE id=?`, resolvedAt, incidentID)
	return err
}
func (r *Store) ListIncidents(ctx context.Context, websiteID *int64, state string, limit int, offset int) ([]models.Incident, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	query := `SELECT id, website_id, state, started_at, acknowledged_at, resolved_at, last_failure_reason, created_at, updated_at FROM incidents WHERE 1=1`
	args := []any{}
	if websiteID != nil {
		query += ` AND website_id=?`
		args = append(args, *websiteID)
	}
	if state != "" {
		query += ` AND state=?`
		args = append(args, state)
	}
	query += ` ORDER BY started_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Incident{}
	for rows.Next() {
		var i models.Incident
		var reason sql.NullString
		if err = rows.Scan(&i.ID, &i.WebsiteID, &i.State, &i.StartedAt, &i.AcknowledgedAt, &i.ResolvedAt, &reason, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		if reason.Valid {
			i.LastFailureReason = &reason.String
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
func (r *Store) ListAlertChannels(ctx context.Context) ([]models.AlertChannel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,name,channel_type,target,enabled,created_at,updated_at FROM alert_channels WHERE enabled=1 ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.AlertChannel{}
	for rows.Next() {
		var c models.AlertChannel
		if err = rows.Scan(&c.ID, &c.Name, &c.ChannelType, &c.Target, &c.Enabled, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (r *Store) CreateAlertChannel(ctx context.Context, channel models.AlertChannel) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO alert_channels (name,channel_type,target,enabled) VALUES (?, ?, ?, ?)`, channel.Name, channel.ChannelType, channel.Target, channel.Enabled)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (r *Store) CreateMaintenanceWindow(ctx context.Context, window models.MaintenanceWindow) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO maintenance_windows (website_id,starts_at,ends_at,mute_alerts,reason) VALUES (?, ?, ?, ?, ?)`, window.WebsiteID, window.StartsAt, window.EndsAt, window.MuteAlerts, window.Reason)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (r *Store) IsWebsiteMuted(ctx context.Context, websiteID int64, now time.Time) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM maintenance_windows WHERE mute_alerts=1 AND starts_at<=? AND ends_at>=? AND (website_id IS NULL OR website_id=?)`, now, now, websiteID).Scan(&count)
	return count > 0, err
}
func (r *Store) CreateStatusPage(ctx context.Context, page models.StatusPage) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO status_pages (slug,title,is_public) VALUES (?, ?, ?)`, page.Slug, page.Title, page.IsPublic)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (r *Store) ListStatusPages(ctx context.Context, limit, offset int) ([]models.StatusPage, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,slug,title,is_public,created_at,updated_at FROM status_pages ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.StatusPage{}
	for rows.Next() {
		var p models.StatusPage
		if err = rows.Scan(&p.ID, &p.Slug, &p.Title, &p.IsPublic, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *Store) GetStatusPageBySlug(ctx context.Context, slug string) (*models.StatusPage, error) {
	var p models.StatusPage
	err := r.db.QueryRowContext(ctx, `SELECT id,slug,title,is_public,created_at,updated_at FROM status_pages WHERE slug=? LIMIT 1`, slug).Scan(&p.ID, &p.Slug, &p.Title, &p.IsPublic, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}
func (r *Store) ListWebsitesByStatusPage(ctx context.Context, pageID int64) ([]models.Website, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, url, health_check_url, check_interval_seconds, monitor_type, expected_keyword, tls_expiry_threshold_days, heartbeat_grace_seconds, status, last_checked_at, last_heartbeat_received_at, next_check_at, last_status_code, last_latency_ms, status_page_id, created_at, updated_at FROM websites WHERE status_page_id=? ORDER BY id DESC`, pageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Website{}
	for rows.Next() {
		var w models.Website
		if err = scanWebsite(rows, &w); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (r *Store) AddEvent(ctx context.Context, eventType string, aggregateID int64, dedupeKey string, payload []byte, availableAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO outbox_events (event_type, aggregate_id, dedupe_key, payload, status, available_at) VALUES (?, ?, ?, ?, 'pending', ?)`, eventType, aggregateID, dedupeKey, payload, availableAt)
	if err != nil {
		if isDuplicate(err) {
			return nil
		}
		return err
	}
	return nil
}
func (r *Store) FetchPending(ctx context.Context, limit int) ([]models.OutboxEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,event_type,aggregate_id,dedupe_key,payload,status,available_at,processed_at,last_error,retry_count,created_at,last_attempted_at FROM outbox_events WHERE status='pending' AND available_at <= UTC_TIMESTAMP() ORDER BY id ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.OutboxEvent{}
	for rows.Next() {
		var e models.OutboxEvent
		var processed, lastAttempt sql.NullTime
		var lastErr sql.NullString
		if err = rows.Scan(&e.ID, &e.EventType, &e.AggregateID, &e.DedupeKey, &e.Payload, &e.Status, &e.AvailableAt, &processed, &lastErr, &e.RetryCount, &e.CreatedAt, &lastAttempt); err != nil {
			return nil, err
		}
		if processed.Valid {
			t := processed.Time
			e.ProcessedAt = &t
		}
		if lastAttempt.Valid {
			t := lastAttempt.Time
			e.LastAttemptedAt = &t
		}
		if lastErr.Valid {
			s := lastErr.String
			e.LastError = &s
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
func (r *Store) MarkProcessed(ctx context.Context, eventID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE outbox_events SET status='processed', processed_at=UTC_TIMESTAMP(), last_error=NULL, last_attempted_at=UTC_TIMESTAMP() WHERE id=?`, eventID)
	return err
}
func (r *Store) MarkFailed(ctx context.Context, eventID int64, message string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE outbox_events SET retry_count = retry_count + 1, last_error=?, last_attempted_at=UTC_TIMESTAMP() WHERE id=?`, message, eventID)
	return err
}

func nullableString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
func isDuplicate(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "for key 'uq_outbox_dedupe'"))
}
