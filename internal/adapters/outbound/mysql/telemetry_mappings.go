package mysql

import (
	"argus/internal/models"
	"context"
)

func (r *Store) CreateTelemetryRouteMapping(ctx context.Context, m models.TelemetryRouteMapping) (int64, error) {
	res, e := r.db.ExecContext(ctx, `INSERT INTO telemetry_route_mappings (project_id,environment_id,route_id,service_name,deployment_environment,http_method,route_template,source) VALUES (?,?,?,?,?,?,?,?)`, m.ProjectID, m.EnvironmentID, m.RouteID, m.ServiceName, m.DeploymentEnvironment, m.HTTPMethod, m.RouteTemplate, m.Source)
	if e != nil {
		return 0, e
	}
	return res.LastInsertId()
}
func (r *Store) ListTelemetryRouteMappings(ctx context.Context, pid int64) ([]models.TelemetryRouteMapping, error) {
	rows, e := r.db.QueryContext(ctx, `SELECT id,project_id,environment_id,route_id,service_name,deployment_environment,http_method,route_template,source,created_at,updated_at FROM telemetry_route_mappings WHERE project_id=? ORDER BY id DESC`, pid)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []models.TelemetryRouteMapping{}
	for rows.Next() {
		var m models.TelemetryRouteMapping
		if e = rows.Scan(&m.ID, &m.ProjectID, &m.EnvironmentID, &m.RouteID, &m.ServiceName, &m.DeploymentEnvironment, &m.HTTPMethod, &m.RouteTemplate, &m.Source, &m.CreatedAt, &m.UpdatedAt); e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (r *Store) DeleteTelemetryRouteMapping(ctx context.Context, pid, id int64) error {
	_, e := r.db.ExecContext(ctx, `DELETE FROM telemetry_route_mappings WHERE id=? AND project_id=?`, id, pid)
	return e
}
