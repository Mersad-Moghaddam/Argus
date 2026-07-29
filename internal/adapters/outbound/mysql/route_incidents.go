package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"argus/internal/models"
)

func (r *Store) GetOpenRouteIncident(ctx context.Context, routeID int64) (*models.RouteIncident, error) {
	var i models.RouteIncident
	var reason, evidence sql.NullString
	var acknowledgedAt sql.NullTime
	var acknowledgedBy sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT id, route_id, project_id, source, source_key, state, started_at, acknowledged_at, acknowledged_by_user_id, resolved_at, failure_count, last_failure_reason, evidence, created_at, updated_at
		FROM route_incidents WHERE route_id=? AND state IN ('open','acknowledged') ORDER BY started_at DESC LIMIT 1`, routeID).
		Scan(&i.ID, &i.RouteID, &i.ProjectID, &i.Source, &i.SourceKey, &i.State, &i.StartedAt, &acknowledgedAt, &acknowledgedBy, &i.ResolvedAt, &i.FailureCount, &reason, &evidence, &i.CreatedAt, &i.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	i.LastFailureReason = reason.String
	i.Evidence = evidence.String
	if acknowledgedAt.Valid {
		i.AcknowledgedAt = &acknowledgedAt.Time
	}
	if acknowledgedBy.Valid {
		i.AcknowledgedByID = &acknowledgedBy.Int64
	}
	return &i, nil
}

func (r *Store) CreateRouteIncident(ctx context.Context, routeID, projectID int64, source, sourceKey, reason, evidence string, startedAt time.Time) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO route_incidents (route_id, project_id, source, source_key, state, started_at, failure_count, last_failure_reason, evidence)
		VALUES (?, ?, ?, ?, 'open', ?, 1, ?, ?)`, routeID, projectID, source, sourceKey, startedAt, nullableString(reason), nullableString(evidence))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Store) GetRouteIncident(ctx context.Context, projectID, incidentID int64) (*models.RouteIncident, error) {
	var i models.RouteIncident
	var reason, evidence sql.NullString
	var acknowledgedAt sql.NullTime
	var acknowledgedBy sql.NullInt64
	err := r.db.QueryRowContext(ctx, `SELECT id, route_id, project_id, source, source_key, state, started_at, acknowledged_at, acknowledged_by_user_id, resolved_at, failure_count, last_failure_reason, evidence, created_at, updated_at FROM route_incidents WHERE project_id=? AND id=? LIMIT 1`, projectID, incidentID).Scan(&i.ID, &i.RouteID, &i.ProjectID, &i.Source, &i.SourceKey, &i.State, &i.StartedAt, &acknowledgedAt, &acknowledgedBy, &i.ResolvedAt, &i.FailureCount, &reason, &evidence, &i.CreatedAt, &i.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	i.LastFailureReason, i.Evidence = reason.String, evidence.String
	if acknowledgedAt.Valid {
		i.AcknowledgedAt = &acknowledgedAt.Time
	}
	if acknowledgedBy.Valid {
		i.AcknowledgedByID = &acknowledgedBy.Int64
	}
	return &i, nil
}

func (r *Store) AcknowledgeRouteIncident(ctx context.Context, incidentID, userID int64, acknowledgedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE route_incidents SET state='acknowledged', acknowledged_at=COALESCE(acknowledged_at, ?), acknowledged_by_user_id=COALESCE(acknowledged_by_user_id, ?), updated_at=NOW() WHERE id=? AND state='open'`, acknowledgedAt, userID, incidentID)
	return err
}

func (r *Store) ResolveRouteIncident(ctx context.Context, incidentID int64, resolvedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE route_incidents SET state='resolved', resolved_at=?, updated_at=NOW() WHERE id=? AND state IN ('open','acknowledged')`, resolvedAt, incidentID)
	return err
}

func (r *Store) ListRouteIncidents(ctx context.Context, projectID int64, routeID *int64, state string, limit, offset int) ([]models.RouteIncident, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	where := `WHERE project_id = ?`
	args := []any{projectID}
	if routeID != nil {
		where += ` AND route_id = ?`
		args = append(args, *routeID)
	}
	if state != "" {
		where += ` AND state = ?`
		args = append(args, state)
	}
	query := fmt.Sprintf(`SELECT id, route_id, project_id, source, source_key, state, started_at, acknowledged_at, acknowledged_by_user_id, resolved_at, failure_count, last_failure_reason, evidence, created_at, updated_at
		FROM route_incidents %s ORDER BY started_at DESC LIMIT ? OFFSET ?`, where)
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.RouteIncident{}
	for rows.Next() {
		var i models.RouteIncident
		var reason, evidence sql.NullString
		var acknowledgedAt sql.NullTime
		var acknowledgedBy sql.NullInt64
		if err = rows.Scan(&i.ID, &i.RouteID, &i.ProjectID, &i.Source, &i.SourceKey, &i.State, &i.StartedAt, &acknowledgedAt, &acknowledgedBy, &i.ResolvedAt, &i.FailureCount, &reason, &evidence, &i.CreatedAt, &i.UpdatedAt); err != nil {
			return nil, err
		}
		i.LastFailureReason = reason.String
		i.Evidence = evidence.String
		if acknowledgedAt.Valid {
			i.AcknowledgedAt = &acknowledgedAt.Time
		}
		if acknowledgedBy.Valid {
			i.AcknowledgedByID = &acknowledgedBy.Int64
		}
		out = append(out, i)
	}
	return out, rows.Err()
}
