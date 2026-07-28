package mysql

import (
	"context"
	"database/sql"
	"time"

	"argus/internal/models"
)

const heartbeatMonitorColumns = `id, project_id, environment_id, created_by_user_id, name, token_prefix, token_hash, expected_interval_seconds, grace_period_seconds, revoked_at, last_received_at, last_outcome, created_at, updated_at`

func scanHeartbeatMonitor(row interface{ Scan(dest ...any) error }, monitor *models.HeartbeatMonitor) error {
	var revokedAt, lastReceivedAt sql.NullTime
	var lastOutcome sql.NullString
	if err := row.Scan(&monitor.ID, &monitor.ProjectID, &monitor.EnvironmentID, &monitor.CreatedByUserID,
		&monitor.Name, &monitor.TokenPrefix, &monitor.TokenHash, &monitor.ExpectedIntervalSeconds, &monitor.GracePeriodSeconds,
		&revokedAt, &lastReceivedAt, &lastOutcome, &monitor.CreatedAt, &monitor.UpdatedAt); err != nil {
		return err
	}
	if revokedAt.Valid {
		monitor.RevokedAt = &revokedAt.Time
	}
	if lastReceivedAt.Valid {
		monitor.LastReceivedAt = &lastReceivedAt.Time
	}
	if lastOutcome.Valid {
		monitor.LastOutcome = lastOutcome.String
	}
	return nil
}

func (r *Store) CreateHeartbeatMonitor(ctx context.Context, monitor models.HeartbeatMonitor) (int64, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO heartbeat_monitors
		(project_id, environment_id, created_by_user_id, name, token_prefix, token_hash, expected_interval_seconds, grace_period_seconds)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, monitor.ProjectID, monitor.EnvironmentID, monitor.CreatedByUserID, monitor.Name,
		monitor.TokenPrefix, monitor.TokenHash, monitor.ExpectedIntervalSeconds, monitor.GracePeriodSeconds)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *Store) ListHeartbeatMonitors(ctx context.Context, projectID int64) ([]models.HeartbeatMonitor, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+heartbeatMonitorColumns+` FROM heartbeat_monitors WHERE project_id=? ORDER BY created_at DESC, id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.HeartbeatMonitor{}
	for rows.Next() {
		var monitor models.HeartbeatMonitor
		if err = scanHeartbeatMonitor(rows, &monitor); err != nil {
			return nil, err
		}
		items = append(items, monitor)
	}
	return items, rows.Err()
}

func (r *Store) GetHeartbeatMonitorByID(ctx context.Context, id int64) (*models.HeartbeatMonitor, error) {
	var monitor models.HeartbeatMonitor
	err := scanHeartbeatMonitor(r.db.QueryRowContext(ctx, `SELECT `+heartbeatMonitorColumns+` FROM heartbeat_monitors WHERE id=? LIMIT 1`, id), &monitor)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &monitor, nil
}

func (r *Store) GetHeartbeatMonitorByHash(ctx context.Context, tokenHash []byte) (*models.HeartbeatMonitor, error) {
	var monitor models.HeartbeatMonitor
	err := scanHeartbeatMonitor(r.db.QueryRowContext(ctx, `SELECT `+heartbeatMonitorColumns+` FROM heartbeat_monitors WHERE token_hash=? LIMIT 1`, tokenHash), &monitor)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &monitor, nil
}

func (r *Store) RevokeHeartbeatMonitor(ctx context.Context, id int64, revokedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE heartbeat_monitors SET revoked_at=COALESCE(revoked_at, ?), updated_at=UTC_TIMESTAMP() WHERE id=?`, revokedAt, id)
	return err
}

func (r *Store) TouchHeartbeatMonitor(ctx context.Context, id int64, receivedAt time.Time, outcome string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE heartbeat_monitors SET last_received_at=?, last_outcome=?, updated_at=UTC_TIMESTAMP() WHERE id=? AND revoked_at IS NULL`, receivedAt, outcome, id)
	return err
}

func (r *Store) RecordHeartbeatReceipt(ctx context.Context, receipt models.HeartbeatReceipt) (bool, error) {
	result, err := r.db.ExecContext(ctx, `INSERT IGNORE INTO heartbeat_receipts (monitor_id, idempotency_key_hash, outcome, received_at) VALUES (?, UNHEX(SHA2(?, 256)), ?, ?)`, receipt.MonitorID, receipt.IdempotencyKey, receipt.Outcome, receipt.ReceivedAt)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}
