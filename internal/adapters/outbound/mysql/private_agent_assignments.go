package mysql

import (
	"context"
	"database/sql"
	"time"

	"argus/internal/models"
)

func (r *Store) CreatePrivateAgentAssignment(ctx context.Context, a models.PrivateAgentAssignment) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO private_agent_assignments (project_id,environment_id,route_id,name,method,target,interval_seconds,timeout_ms,enabled,created_by_user_id)
		VALUES (?,?,?,?,?,?,?,?,?,?)`, a.ProjectID, a.EnvironmentID, a.RouteID, a.Name, a.Method, a.Target, a.IntervalSecs, a.TimeoutMS, a.Enabled, a.CreatedByID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Store) ListPrivateAgentAssignments(ctx context.Context, projectID int64) ([]models.PrivateAgentAssignment, error) {
	return r.listPrivateAgentAssignments(ctx, `WHERE project_id=?`, projectID)
}

func (r *Store) ListPrivateAgentAssignmentsForEnvironment(ctx context.Context, projectID, environmentID int64) ([]models.PrivateAgentAssignment, error) {
	return r.listPrivateAgentAssignments(ctx, `WHERE project_id=? AND environment_id=?`, projectID, environmentID)
}

func (r *Store) listPrivateAgentAssignments(ctx context.Context, where string, args ...any) ([]models.PrivateAgentAssignment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,project_id,environment_id,route_id,name,method,target,interval_seconds,timeout_ms,enabled,created_by_user_id,created_at,updated_at,revoked_at FROM private_agent_assignments `+where+` ORDER BY id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.PrivateAgentAssignment{}
	for rows.Next() {
		var a models.PrivateAgentAssignment
		var revoked sql.NullTime
		if err = rows.Scan(&a.ID, &a.ProjectID, &a.EnvironmentID, &a.RouteID, &a.Name, &a.Method, &a.Target, &a.IntervalSecs, &a.TimeoutMS, &a.Enabled, &a.CreatedByID, &a.CreatedAt, &a.UpdatedAt, &revoked); err != nil {
			return nil, err
		}
		if revoked.Valid {
			a.RevokedAt = &revoked.Time
		}
		items = append(items, a)
	}
	return items, rows.Err()
}

func (r *Store) RevokePrivateAgentAssignment(ctx context.Context, projectID, id int64, revokedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE private_agent_assignments SET revoked_at=?, enabled=0 WHERE project_id=? AND id=? AND revoked_at IS NULL`, revokedAt, projectID, id)
	return err
}
