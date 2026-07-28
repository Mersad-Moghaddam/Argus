package mysql

import (
	"context"
	"database/sql"
	"time"

	"argus/internal/models"
)

const privateAgentColumns = `id,project_id,environment_id,created_by_user_id,name,token_prefix,token_hash,version,last_seen_at,revoked_at,created_at,updated_at`

func scanPrivateAgent(row interface{ Scan(dest ...any) error }, agent *models.PrivateAgent) error {
	var seen, revoked sql.NullTime
	if err := row.Scan(&agent.ID, &agent.ProjectID, &agent.EnvironmentID, &agent.CreatedByUserID, &agent.Name, &agent.TokenPrefix, &agent.TokenHash, &agent.Version, &seen, &revoked, &agent.CreatedAt, &agent.UpdatedAt); err != nil {
		return err
	}
	if seen.Valid {
		agent.LastSeenAt = &seen.Time
	}
	if revoked.Valid {
		agent.RevokedAt = &revoked.Time
	}
	return nil
}
func (r *Store) CreatePrivateAgent(ctx context.Context, agent models.PrivateAgent) (int64, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO private_agents (project_id,environment_id,created_by_user_id,name,token_prefix,token_hash,version) VALUES (?,?,?,?,?,?,?)`, agent.ProjectID, agent.EnvironmentID, agent.CreatedByUserID, agent.Name, agent.TokenPrefix, agent.TokenHash, agent.Version)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
func (r *Store) ListPrivateAgents(ctx context.Context, projectID int64) ([]models.PrivateAgent, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+privateAgentColumns+` FROM private_agents WHERE project_id=? ORDER BY created_at DESC,id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.PrivateAgent{}
	for rows.Next() {
		var a models.PrivateAgent
		if err = scanPrivateAgent(rows, &a); err != nil {
			return nil, err
		}
		items = append(items, a)
	}
	return items, rows.Err()
}
func (r *Store) GetPrivateAgentByHash(ctx context.Context, hash []byte) (*models.PrivateAgent, error) {
	var a models.PrivateAgent
	err := scanPrivateAgent(r.db.QueryRowContext(ctx, `SELECT `+privateAgentColumns+` FROM private_agents WHERE token_hash=? LIMIT 1`, hash), &a)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
func (r *Store) RevokePrivateAgent(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE private_agents SET revoked_at=COALESCE(revoked_at,?),updated_at=UTC_TIMESTAMP() WHERE id=?`, at, id)
	return err
}
func (r *Store) TouchPrivateAgent(ctx context.Context, id int64, version string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE private_agents SET last_seen_at=?,version=?,updated_at=UTC_TIMESTAMP() WHERE id=? AND revoked_at IS NULL`, at, version, id)
	return err
}
