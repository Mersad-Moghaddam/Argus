package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"argus/internal/models"
)

func scanProject(row interface{ Scan(dest ...any) error }, p *models.Project) error {
	var lastCheck, metricsUpdated sql.NullTime
	err := row.Scan(&p.ID, &p.OwnerUserID, &p.Name, &p.Slug, &p.Description, &p.Status,
		&p.DefaultIntervalSeconds, &p.DefaultTimeoutMS, &p.DefaultRetries, &p.FailureThreshold, &p.RecoverySuccessThreshold,
		&p.RoutesTotal, &p.RoutesHealthy, &p.RoutesDegraded, &p.RoutesFailing, &p.RoutesDisabled, &p.RoutesUnknown,
		&p.Uptime24hPct, &p.AvgLatency24hMS, &p.Checks24h, &p.Failures24h, &p.OpenIncidents,
		&lastCheck, &metricsUpdated, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return err
	}
	if lastCheck.Valid {
		p.LastCheckAt = &lastCheck.Time
	}
	if metricsUpdated.Valid {
		p.MetricsUpdatedAt = &metricsUpdated.Time
	}
	return nil
}

const projectColumns = `id, owner_user_id, name, slug, description, status,
	default_interval_seconds, default_timeout_ms, default_retries, failure_threshold, recovery_success_threshold,
	routes_total, routes_healthy, routes_degraded, routes_failing, routes_disabled, routes_unknown,
	uptime_24h_pct, avg_latency_24h_ms, checks_24h, failures_24h, open_incidents,
	last_check_at, metrics_updated_at, created_at, updated_at`

func (r *Store) CreateProject(ctx context.Context, project models.Project, ownerUserID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `INSERT INTO projects (owner_user_id, name, slug, description, status, default_interval_seconds, default_timeout_ms, default_retries, failure_threshold, recovery_success_threshold)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ownerUserID, project.Name, project.Slug, project.Description, project.Status,
		project.DefaultIntervalSeconds, project.DefaultTimeoutMS, project.DefaultRetries, project.FailureThreshold, project.RecoverySuccessThreshold)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO project_members (project_id, user_id, role) VALUES (?, ?, 'owner')`, id, ownerUserID); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Store) UpdateProject(ctx context.Context, project models.Project) error {
	_, err := r.db.ExecContext(ctx, `UPDATE projects SET name=?, description=?, default_interval_seconds=?, default_timeout_ms=?, default_retries=?, failure_threshold=?, recovery_success_threshold=?, updated_at=NOW() WHERE id=?`,
		project.Name, project.Description, project.DefaultIntervalSeconds, project.DefaultTimeoutMS, project.DefaultRetries, project.FailureThreshold, project.RecoverySuccessThreshold, project.ID)
	return err
}

func (r *Store) SetProjectStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE projects SET status=?, updated_at=NOW() WHERE id=?`, status, id)
	return err
}

func (r *Store) DeleteProject(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM projects WHERE id=?`, id)
	return err
}

func (r *Store) GetProjectByID(ctx context.Context, id int64) (*models.Project, error) {
	var p models.Project
	row := r.db.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE id=? LIMIT 1`, id)
	if err := scanProject(row, &p); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *Store) ListProjects(ctx context.Context, userID int64, filter models.ProjectFilter) ([]models.Project, int, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	where := `WHERE p.id IN (SELECT project_id FROM project_members WHERE user_id = ?)`
	args := []any{userID}
	if filter.Status != "" {
		where += ` AND p.status = ?`
		args = append(args, filter.Status)
	}
	if filter.Search != "" {
		where += ` AND (p.name LIKE ? OR p.slug LIKE ? OR p.description LIKE ?)`
		like := "%" + filter.Search + "%"
		args = append(args, like, like, like)
	}

	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM projects p %s`, where)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(`SELECT %s FROM projects p %s ORDER BY p.updated_at DESC LIMIT ? OFFSET ?`, projectColumns, where)
	args = append(args, limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.Project{}
	for rows.Next() {
		var p models.Project
		if err = scanProject(rows, &p); err != nil {
			return nil, 0, err
		}
		out = append(out, p)
	}
	return out, total, rows.Err()
}

func (r *Store) GetProjectMember(ctx context.Context, projectID, userID int64) (*models.ProjectMember, error) {
	var m models.ProjectMember
	err := r.db.QueryRowContext(ctx, `SELECT id, project_id, user_id, role, created_at FROM project_members WHERE project_id=? AND user_id=? LIMIT 1`, projectID, userID).
		Scan(&m.ID, &m.ProjectID, &m.UserID, &m.Role, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &m, err
}

func (r *Store) AddProjectMember(ctx context.Context, member models.ProjectMember) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO project_members (project_id, user_id, role) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE role=VALUES(role)`, member.ProjectID, member.UserID, member.Role)
	return err
}
