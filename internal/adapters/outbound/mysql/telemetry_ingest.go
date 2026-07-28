package mysql

import (
	"context"

	"argus/internal/models"
)

func (r *Store) RecordTelemetryIngress(ctx context.Context, record models.TelemetryIngressRecord) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO telemetry_ingest_records
		(project_id, environment_id, credential_id, signal_type, service_name, deployment_environment, item_count, received_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ProjectID, record.EnvironmentID, record.CredentialID, record.SignalType, record.ServiceName,
		record.DeploymentEnvironment, record.ItemCount, record.ReceivedAt)
	return err
}

func (r *Store) ListTelemetryIngress(ctx context.Context, projectID int64, limit int) ([]models.TelemetryIngressRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, project_id, environment_id, credential_id, signal_type, service_name, deployment_environment, item_count, received_at
		FROM telemetry_ingest_records WHERE project_id=? ORDER BY received_at DESC, id DESC LIMIT ?`, projectID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []models.TelemetryIngressRecord{}
	for rows.Next() {
		var record models.TelemetryIngressRecord
		if err = rows.Scan(&record.ID, &record.ProjectID, &record.EnvironmentID, &record.CredentialID, &record.SignalType,
			&record.ServiceName, &record.DeploymentEnvironment, &record.ItemCount, &record.ReceivedAt); err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	return items, rows.Err()
}
