package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"argus/internal/models"
)

const projectIncidentColumns = `id,project_id,source,source_key,state,title,evidence,started_at,acknowledged_at,acknowledged_by_user_id,resolved_at`

func scanProjectIncident(row interface{ Scan(...any) error }, i *models.ProjectIncident) error {
	var evidence sql.NullString
	var ack, resolved sql.NullTime
	var by sql.NullInt64
	if err := row.Scan(&i.ID, &i.ProjectID, &i.Source, &i.SourceKey, &i.State, &i.Title, &evidence, &i.StartedAt, &ack, &by, &resolved); err != nil {
		return err
	}
	i.Evidence = evidence.String
	if ack.Valid {
		i.AcknowledgedAt = &ack.Time
	}
	if by.Valid {
		i.AcknowledgedByID = &by.Int64
	}
	if resolved.Valid {
		i.ResolvedAt = &resolved.Time
	}
	return nil
}
func (r *Store) GetOpenProjectIncident(ctx context.Context, projectID int64, source, key string) (*models.ProjectIncident, error) {
	var i models.ProjectIncident
	err := scanProjectIncident(r.db.QueryRowContext(ctx, `SELECT `+projectIncidentColumns+` FROM project_incidents WHERE project_id=? AND source=? AND source_key=? AND state IN ('open','acknowledged') ORDER BY id DESC LIMIT 1`, projectID, source, key), &i)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &i, nil
}
func (r *Store) CreateProjectIncident(ctx context.Context, i models.ProjectIncident) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO project_incidents (project_id,source,source_key,title,evidence,started_at) VALUES (?,?,?,?,?,?)`, i.ProjectID, i.Source, i.SourceKey, i.Title, nullableString(i.Evidence), i.StartedAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
func (r *Store) ResolveProjectIncident(ctx context.Context, id int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE project_incidents SET state='resolved',resolved_at=?,updated_at=UTC_TIMESTAMP() WHERE id=? AND state IN ('open','acknowledged')`, at, id)
	return err
}
func (r *Store) AcknowledgeProjectIncident(ctx context.Context, pid, id, uid int64, at time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE project_incidents SET state='acknowledged',acknowledged_at=COALESCE(acknowledged_at,?),acknowledged_by_user_id=COALESCE(acknowledged_by_user_id,?) WHERE id=? AND project_id=? AND state='open'`, at, uid, id, pid)
	return err
}
func (r *Store) ListProjectIncidents(ctx context.Context, pid int64, state string, limit, offset int) ([]models.ProjectIncident, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	where := "WHERE project_id=?"
	args := []any{pid}
	if state != "" {
		where += " AND state=?"
		args = append(args, state)
	}
	q := fmt.Sprintf(`SELECT %s FROM project_incidents %s ORDER BY started_at DESC,id DESC LIMIT ? OFFSET ?`, projectIncidentColumns, where)
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.ProjectIncident{}
	for rows.Next() {
		var i models.ProjectIncident
		if err = scanProjectIncident(rows, &i); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
