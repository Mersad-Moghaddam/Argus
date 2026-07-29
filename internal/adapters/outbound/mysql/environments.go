package mysql

import (
	"context"

	"argus/internal/models"
)

func (r *Store) ListProjectEnvironments(ctx context.Context, projectID int64) ([]models.ProjectEnvironment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, project_id, name, canonical_base_url, canonical_origin, is_default, created_at, updated_at FROM project_environments WHERE project_id=? ORDER BY is_default DESC, name ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ProjectEnvironment
	for rows.Next() {
		var env models.ProjectEnvironment
		if err := rows.Scan(&env.ID, &env.ProjectID, &env.Name, &env.CanonicalBaseURL, &env.CanonicalOrigin, &env.IsDefault, &env.CreatedAt, &env.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, env)
	}
	return out, rows.Err()
}

func (r *Store) CreateProjectEnvironment(ctx context.Context, env models.ProjectEnvironment) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO project_environments (project_id, name, canonical_base_url, canonical_origin, is_default) VALUES (?, ?, ?, ?, ?)`, env.ProjectID, env.Name, env.CanonicalBaseURL, env.CanonicalOrigin, env.IsDefault)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Store) UpdateProjectEnvironment(ctx context.Context, env models.ProjectEnvironment) error {
	_, err := r.db.ExecContext(ctx, `UPDATE project_environments SET name=?, canonical_base_url=?, canonical_origin=? WHERE id=? AND project_id=?`, env.Name, env.CanonicalBaseURL, env.CanonicalOrigin, env.ID, env.ProjectID)
	return err
}
