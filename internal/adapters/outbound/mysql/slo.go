package mysql

import (
	"context"
	"database/sql"
	"encoding/json"

	"argus/internal/models"
)

func (r *Store) CreateSLODefinition(ctx context.Context, definition models.SLODefinition) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	definition.Version = 1
	result, err := tx.ExecContext(ctx, `INSERT INTO slo_definitions (project_id,created_by_user_id,name,sli_kind,target_percent,window_seconds,latency_threshold_ms,min_events,short_window_seconds,short_burn_rate,long_window_seconds,long_burn_rate,paused,version) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, definition.ProjectID, definition.CreatedByUserID, definition.Name, definition.SLIKind, definition.TargetPercent, definition.WindowSeconds, definition.LatencyThresholdMS, definition.MinEvents, definition.ShortWindowSeconds, definition.ShortBurnRate, definition.LongWindowSeconds, definition.LongBurnRate, definition.Paused, definition.Version)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	definition.ID = id
	payload, err := json.Marshal(definition)
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO slo_definition_versions (slo_id,project_id,version,definition_json,created_by_user_id) VALUES (?,?,?,?,?)`, id, definition.ProjectID, definition.Version, payload, definition.CreatedByUserID); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *Store) GetSLODefinition(ctx context.Context, projectID, id int64) (*models.SLODefinition, error) {
	row := r.db.QueryRowContext(ctx, sloDefinitionSelect+` WHERE project_id=? AND id=?`, projectID, id)
	definition, err := scanSLODefinition(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return definition, err
}

func (r *Store) ListSLODefinitions(ctx context.Context, projectID int64) ([]models.SLODefinition, error) {
	rows, err := r.db.QueryContext(ctx, sloDefinitionSelect+` WHERE project_id=? ORDER BY id DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.SLODefinition{}
	for rows.Next() {
		item, err := scanSLODefinition(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

const sloDefinitionSelect = `SELECT id,project_id,created_by_user_id,name,sli_kind,target_percent,window_seconds,latency_threshold_ms,min_events,short_window_seconds,short_burn_rate,long_window_seconds,long_burn_rate,paused,version,created_at,updated_at FROM slo_definitions`

type scanner interface{ Scan(...any) error }

func scanSLODefinition(row scanner) (*models.SLODefinition, error) {
	var item models.SLODefinition
	err := row.Scan(&item.ID, &item.ProjectID, &item.CreatedByUserID, &item.Name, &item.SLIKind, &item.TargetPercent, &item.WindowSeconds, &item.LatencyThresholdMS, &item.MinEvents, &item.ShortWindowSeconds, &item.ShortBurnRate, &item.LongWindowSeconds, &item.LongBurnRate, &item.Paused, &item.Version, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *Store) RecordSLOEvaluation(ctx context.Context, item models.SLOEvaluation) (int64, error) {
	result, err := r.db.ExecContext(ctx, `INSERT INTO slo_evaluations (slo_id,project_id,definition_version,status,observed_percent,error_budget_remaining,burn_rate,good_events,total_events,window_started_at,window_ended_at,observed_at,provenance,evaluated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.SLOID, item.ProjectID, item.DefinitionVersion, item.Status, item.ObservedPercent, item.ErrorBudgetRemaining, item.BurnRate, item.GoodEvents, item.TotalEvents, item.WindowStartedAt, item.WindowEndedAt, item.ObservedAt, item.Provenance, item.EvaluatedAt)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *Store) ListSLOEvaluations(ctx context.Context, projectID, sloID int64, limit int) ([]models.SLOEvaluation, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,slo_id,project_id,definition_version,status,observed_percent,error_budget_remaining,burn_rate,good_events,total_events,window_started_at,window_ended_at,observed_at,provenance,evaluated_at FROM slo_evaluations WHERE project_id=? AND slo_id=? ORDER BY evaluated_at DESC, id DESC LIMIT ?`, projectID, sloID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.SLOEvaluation{}
	for rows.Next() {
		var item models.SLOEvaluation
		var observed, budget, burn sql.NullFloat64
		var observedAt sql.NullTime
		if err = rows.Scan(&item.ID, &item.SLOID, &item.ProjectID, &item.DefinitionVersion, &item.Status, &observed, &budget, &burn, &item.GoodEvents, &item.TotalEvents, &item.WindowStartedAt, &item.WindowEndedAt, &observedAt, &item.Provenance, &item.EvaluatedAt); err != nil {
			return nil, err
		}
		if observed.Valid {
			value := observed.Float64
			item.ObservedPercent = &value
		}
		if budget.Valid {
			value := budget.Float64
			item.ErrorBudgetRemaining = &value
		}
		if burn.Valid {
			value := burn.Float64
			item.BurnRate = &value
		}
		if observedAt.Valid {
			value := observedAt.Time
			item.ObservedAt = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
