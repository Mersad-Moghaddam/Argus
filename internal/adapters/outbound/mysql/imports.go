package mysql

import (
	"context"
	"database/sql"
	"encoding/json"

	"argus/internal/models"
)

func (r *Store) CreateImportJob(ctx context.Context, job models.ImportJob) (int64, error) {
	itemsJSON, err := json.Marshal(job.Items)
	if err != nil {
		return 0, err
	}
	res, err := r.db.ExecContext(ctx, `INSERT INTO route_import_jobs (project_id, created_by_user_id, source_type, spec_format, status, parsed_items, total_parsed, created_routes, updated_routes, skipped_routes, removed_routes, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ProjectID, job.CreatedByUserID, job.SourceType, job.SpecFormat, job.Status, string(itemsJSON), job.TotalParsed, job.CreatedRoutes, job.UpdatedRoutes, job.SkippedRoutes, job.RemovedRoutes, nullableString(job.ErrorMessage))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Store) GetImportJob(ctx context.Context, id int64) (*models.ImportJob, error) {
	var job models.ImportJob
	var itemsJSON string
	var errMsg sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, project_id, created_by_user_id, source_type, spec_format, status, parsed_items, total_parsed, created_routes, updated_routes, skipped_routes, removed_routes, error_message, created_at, updated_at
		FROM route_import_jobs WHERE id=? LIMIT 1`, id).
		Scan(&job.ID, &job.ProjectID, &job.CreatedByUserID, &job.SourceType, &job.SpecFormat, &job.Status, &itemsJSON, &job.TotalParsed, &job.CreatedRoutes, &job.UpdatedRoutes, &job.SkippedRoutes, &job.RemovedRoutes, &errMsg, &job.CreatedAt, &job.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job.ErrorMessage = errMsg.String
	if itemsJSON != "" {
		_ = json.Unmarshal([]byte(itemsJSON), &job.Items)
	}
	return &job, nil
}

func (r *Store) UpdateImportJob(ctx context.Context, job models.ImportJob) error {
	itemsJSON, err := json.Marshal(job.Items)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE route_import_jobs SET status=?, parsed_items=?, total_parsed=?, created_routes=?, updated_routes=?, skipped_routes=?, removed_routes=?, error_message=?, updated_at=NOW()
		WHERE id=?`,
		job.Status, string(itemsJSON), job.TotalParsed, job.CreatedRoutes, job.UpdatedRoutes, job.SkippedRoutes, job.RemovedRoutes, nullableString(job.ErrorMessage), job.ID)
	return err
}
