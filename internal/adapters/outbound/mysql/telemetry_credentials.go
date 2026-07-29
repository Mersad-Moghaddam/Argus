package mysql

import (
	"context"
	"database/sql"
	"time"

	"argus/internal/models"
)

const telemetryCredentialColumns = `id, project_id, environment_id, created_by_user_id, name, token_prefix, token_hash, scopes, rate_limit_per_minute, expires_at, revoked_at, last_used_at, created_at, updated_at`

func scanTelemetryCredential(row interface{ Scan(dest ...any) error }, credential *models.TelemetryCredential) error {
	var expiresAt, revokedAt, lastUsedAt sql.NullTime
	if err := row.Scan(&credential.ID, &credential.ProjectID, &credential.EnvironmentID, &credential.CreatedByUserID,
		&credential.Name, &credential.TokenPrefix, &credential.TokenHash, &credential.Scopes, &credential.RateLimitPerMinute,
		&expiresAt, &revokedAt, &lastUsedAt, &credential.CreatedAt, &credential.UpdatedAt); err != nil {
		return err
	}
	if expiresAt.Valid {
		credential.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		credential.RevokedAt = &revokedAt.Time
	}
	if lastUsedAt.Valid {
		credential.LastUsedAt = &lastUsedAt.Time
	}
	return nil
}

func (r *Store) CreateTelemetryCredential(ctx context.Context, credential models.TelemetryCredential) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO telemetry_credentials
		(project_id, environment_id, created_by_user_id, name, token_prefix, token_hash, scopes, rate_limit_per_minute, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		credential.ProjectID, credential.EnvironmentID, credential.CreatedByUserID, credential.Name, credential.TokenPrefix,
		credential.TokenHash, credential.Scopes, credential.RateLimitPerMinute, credential.ExpiresAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Store) ListTelemetryCredentials(ctx context.Context, projectID int64) ([]models.TelemetryCredential, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+telemetryCredentialColumns+` FROM telemetry_credentials WHERE project_id=? ORDER BY created_at DESC, id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.TelemetryCredential{}
	for rows.Next() {
		var credential models.TelemetryCredential
		if err = scanTelemetryCredential(rows, &credential); err != nil {
			return nil, err
		}
		items = append(items, credential)
	}
	return items, rows.Err()
}

func (r *Store) GetTelemetryCredentialByID(ctx context.Context, id int64) (*models.TelemetryCredential, error) {
	var credential models.TelemetryCredential
	err := scanTelemetryCredential(r.db.QueryRowContext(ctx, `SELECT `+telemetryCredentialColumns+` FROM telemetry_credentials WHERE id=? LIMIT 1`, id), &credential)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &credential, nil
}

func (r *Store) GetTelemetryCredentialByHash(ctx context.Context, tokenHash []byte) (*models.TelemetryCredential, error) {
	var credential models.TelemetryCredential
	err := scanTelemetryCredential(r.db.QueryRowContext(ctx, `SELECT `+telemetryCredentialColumns+` FROM telemetry_credentials WHERE token_hash=? LIMIT 1`, tokenHash), &credential)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &credential, nil
}

func (r *Store) RevokeTelemetryCredential(ctx context.Context, id int64, revokedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE telemetry_credentials SET revoked_at=COALESCE(revoked_at, ?), updated_at=UTC_TIMESTAMP() WHERE id=?`, revokedAt, id)
	return err
}

func (r *Store) TouchTelemetryCredential(ctx context.Context, id int64, usedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE telemetry_credentials SET last_used_at=?, updated_at=UTC_TIMESTAMP() WHERE id=? AND revoked_at IS NULL`, usedAt, id)
	return err
}
