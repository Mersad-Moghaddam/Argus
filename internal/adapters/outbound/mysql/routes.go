package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"argus/internal/models"
)

const routeColumns = `id, project_id, method, path, base_url, operation_id, name, summary, description, tags, deprecated,
	parameters, request_body, responses, security, headers, spec_hash, source, enabled,
	monitor_interval_seconds, timeout_ms, retries, expected_status_range, failure_threshold, recovery_successes,
	status, last_checked_at, last_status_code, last_latency_ms, last_failure_reason, consecutive_failures, consecutive_successes,
	next_check_at, uptime_24h_pct, avg_latency_24h_ms, checks_24h, failures_24h, created_at, updated_at`

func scanRoute(row interface{ Scan(dest ...any) error }, rt *models.APIRoute) error {
	var description, tags, parameters, requestBody, responses, security, headers, lastFailureReason sql.NullString
	var lastChecked sql.NullTime
	err := row.Scan(&rt.ID, &rt.ProjectID, &rt.Method, &rt.Path, &rt.BaseURL, &rt.OperationID, &rt.Name, &rt.Summary, &description, &tags, &rt.Deprecated,
		&parameters, &requestBody, &responses, &security, &headers, &rt.SpecHash, &rt.Source, &rt.Enabled,
		&rt.MonitorIntervalSecs, &rt.TimeoutMS, &rt.Retries, &rt.ExpectedStatusRange, &rt.FailureThreshold, &rt.RecoverySuccesses,
		&rt.Status, &lastChecked, &rt.LastStatusCode, &rt.LastLatencyMS, &lastFailureReason, &rt.ConsecutiveFailures, &rt.ConsecutiveSuccesses,
		&rt.NextCheckAt, &rt.Uptime24hPct, &rt.AvgLatency24hMS, &rt.Checks24h, &rt.Failures24h, &rt.CreatedAt, &rt.UpdatedAt)
	if err != nil {
		return err
	}
	rt.Description = description.String
	rt.Parameters = parameters.String
	rt.RequestBody = requestBody.String
	rt.Responses = responses.String
	rt.Security = security.String
	rt.Headers = headers.String
	rt.LastFailureReason = lastFailureReason.String
	if lastChecked.Valid {
		rt.LastCheckedAt = &lastChecked.Time
	}
	if tags.Valid && tags.String != "" {
		_ = json.Unmarshal([]byte(tags.String), &rt.Tags)
	}
	return nil
}

func marshalTags(tags []string) any {
	if len(tags) == 0 {
		return nil
	}
	b, err := json.Marshal(tags)
	if err != nil {
		return nil
	}
	return string(b)
}

func nullableJSON(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func (r *Store) CreateRoute(ctx context.Context, route models.APIRoute) (int64, error) {
	res, err := r.db.ExecContext(ctx, `INSERT INTO api_routes (project_id, method, path, base_url, operation_id, name, summary, description, tags, deprecated,
			parameters, request_body, responses, security, headers, spec_hash, source, enabled,
			monitor_interval_seconds, timeout_ms, retries, expected_status_range, failure_threshold, recovery_successes,
			status, next_check_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		route.ProjectID, route.Method, route.Path, route.BaseURL, route.OperationID, route.Name, route.Summary, nullableJSON(route.Description), marshalTags(route.Tags), route.Deprecated,
		nullableJSON(route.Parameters), nullableJSON(route.RequestBody), nullableJSON(route.Responses), nullableJSON(route.Security), nullableJSON(route.Headers), route.SpecHash, route.Source, route.Enabled,
		route.MonitorIntervalSecs, route.TimeoutMS, route.Retries, route.ExpectedStatusRange, route.FailureThreshold, route.RecoverySuccesses,
		route.Status, route.NextCheckAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *Store) BulkCreateRoutes(ctx context.Context, routes []models.APIRoute) (int, error) {
	if len(routes) == 0 {
		return 0, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO api_routes (project_id, method, path, base_url, operation_id, name, summary, description, tags, deprecated,
			parameters, request_body, responses, security, headers, spec_hash, source, enabled,
			monitor_interval_seconds, timeout_ms, retries, expected_status_range, failure_threshold, recovery_successes,
			status, next_check_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, route := range routes {
		if _, err = stmt.ExecContext(ctx, route.ProjectID, route.Method, route.Path, route.BaseURL, route.OperationID, route.Name, route.Summary, nullableJSON(route.Description), marshalTags(route.Tags), route.Deprecated,
			nullableJSON(route.Parameters), nullableJSON(route.RequestBody), nullableJSON(route.Responses), nullableJSON(route.Security), nullableJSON(route.Headers), route.SpecHash, route.Source, route.Enabled,
			route.MonitorIntervalSecs, route.TimeoutMS, route.Retries, route.ExpectedStatusRange, route.FailureThreshold, route.RecoverySuccesses,
			route.Status, route.NextCheckAt); err != nil {
			return 0, err
		}
		count++
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Store) UpdateRoute(ctx context.Context, route models.APIRoute) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_routes SET name=?, summary=?, description=?, tags=?, deprecated=?,
			parameters=?, request_body=?, responses=?, security=?, headers=?, spec_hash=?, enabled=?,
			monitor_interval_seconds=?, timeout_ms=?, retries=?, expected_status_range=?, failure_threshold=?, recovery_successes=?,
			base_url=?, updated_at=NOW()
		WHERE id=?`,
		route.Name, route.Summary, nullableJSON(route.Description), marshalTags(route.Tags), route.Deprecated,
		nullableJSON(route.Parameters), nullableJSON(route.RequestBody), nullableJSON(route.Responses), nullableJSON(route.Security), nullableJSON(route.Headers), route.SpecHash, route.Enabled,
		route.MonitorIntervalSecs, route.TimeoutMS, route.Retries, route.ExpectedStatusRange, route.FailureThreshold, route.RecoverySuccesses,
		route.BaseURL, route.ID)
	return err
}

// UpdateRouteImportedMetadata updates only spec-derived fields on an
// existing route during re-import. It deliberately never touches
// user-owned monitoring configuration (enabled, interval, timeout, retries,
// thresholds, expected status range) so re-importing a spec can never
// silently clobber a user's monitoring settings.
func (r *Store) UpdateRouteImportedMetadata(ctx context.Context, route models.APIRoute) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_routes SET name=?, summary=?, description=?, tags=?, deprecated=?,
			parameters=?, request_body=?, responses=?, security=?, spec_hash=?, base_url=?, updated_at=NOW()
		WHERE id=?`,
		route.Name, route.Summary, nullableJSON(route.Description), marshalTags(route.Tags), route.Deprecated,
		nullableJSON(route.Parameters), nullableJSON(route.RequestBody), nullableJSON(route.Responses), nullableJSON(route.Security), route.SpecHash, route.BaseURL, route.ID)
	return err
}

func (r *Store) SetRouteEnabled(ctx context.Context, id int64, enabled bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_routes SET enabled=?, updated_at=NOW(),
			status = CASE
				WHEN ? = 0 THEN 'disabled'
				WHEN last_checked_at IS NULL THEN 'unknown'
				WHEN consecutive_failures >= failure_threshold THEN 'failing'
				WHEN consecutive_failures > 0 THEN 'degraded'
				ELSE 'healthy'
			END
		WHERE id=?`, enabled, enabled, id)
	return err
}

func (r *Store) DeleteRoute(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM api_routes WHERE id=?`, id)
	return err
}

func (r *Store) BulkDeleteRoutes(ctx context.Context, projectID int64, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, projectID)
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}
	query := fmt.Sprintf(`DELETE FROM api_routes WHERE project_id=? AND id IN (%s)`, strings.Join(placeholders, ","))
	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *Store) GetRouteByID(ctx context.Context, id int64) (*models.APIRoute, error) {
	var rt models.APIRoute
	row := r.db.QueryRowContext(ctx, `SELECT `+routeColumns+` FROM api_routes WHERE id=? LIMIT 1`, id)
	if err := scanRoute(row, &rt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &rt, nil
}

func (r *Store) GetRouteByMethodPath(ctx context.Context, projectID int64, method, path string) (*models.APIRoute, error) {
	var rt models.APIRoute
	row := r.db.QueryRowContext(ctx, `SELECT `+routeColumns+` FROM api_routes WHERE project_id=? AND method=? AND path=? LIMIT 1`, projectID, method, path)
	if err := scanRoute(row, &rt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &rt, nil
}

func (r *Store) ListAllRouteKeys(ctx context.Context, projectID int64) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, method, path FROM api_routes WHERE project_id=?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var id int64
		var method, path string
		if err = rows.Scan(&id, &method, &path); err != nil {
			return nil, err
		}
		out[method+" "+path] = id
	}
	return out, rows.Err()
}

func (r *Store) ListRouteSpecHashes(ctx context.Context, projectID int64) (map[int64]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, spec_hash FROM api_routes WHERE project_id=?`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]string{}
	for rows.Next() {
		var id int64
		var hash string
		if err = rows.Scan(&id, &hash); err != nil {
			return nil, err
		}
		out[id] = hash
	}
	return out, rows.Err()
}

func (r *Store) ListRoutes(ctx context.Context, filter models.RouteFilter) ([]models.APIRoute, int, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	where := `WHERE project_id = ?`
	args := []any{filter.ProjectID}
	if filter.Search != "" {
		where += ` AND (path LIKE ? OR summary LIKE ? OR operation_id LIKE ? OR name LIKE ?)`
		like := "%" + filter.Search + "%"
		args = append(args, like, like, like, like)
	}
	if filter.Method != "" {
		where += ` AND method = ?`
		args = append(args, filter.Method)
	}
	if filter.Status != "" {
		where += ` AND status = ?`
		args = append(args, filter.Status)
	}
	if filter.Tag != "" {
		where += ` AND JSON_CONTAINS(tags, JSON_QUOTE(?))`
		args = append(args, filter.Tag)
	}
	if filter.Enabled != nil {
		where += ` AND enabled = ?`
		args = append(args, *filter.Enabled)
	}
	if filter.Deprecated != nil {
		where += ` AND deprecated = ?`
		args = append(args, *filter.Deprecated)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM api_routes `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderCol := map[string]string{
		"path": "path", "method": "method", "status": "status",
		"latency": "last_latency_ms", "uptime": "uptime_24h_pct", "updated": "updated_at",
	}[filter.SortBy]
	if orderCol == "" {
		orderCol = "path"
	}
	dir := "ASC"
	if strings.EqualFold(filter.SortDir, "desc") {
		dir = "DESC"
	}

	query := fmt.Sprintf(`SELECT %s FROM api_routes %s ORDER BY %s %s, id ASC LIMIT ? OFFSET ?`, routeColumns, where, orderCol, dir)
	args = append(args, limit, filter.Offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []models.APIRoute{}
	for rows.Next() {
		var rt models.APIRoute
		if err = scanRoute(rows, &rt); err != nil {
			return nil, 0, err
		}
		out = append(out, rt)
	}
	return out, total, rows.Err()
}

func (r *Store) ListDueRoutes(ctx context.Context, now time.Time, limit int, afterID int64) ([]models.APIRoute, error) {
	if limit <= 0 || limit > 1000 {
		limit = 500
	}
	rows, err := r.db.QueryContext(ctx, `SELECT `+routeColumns+` FROM api_routes WHERE enabled=1 AND next_check_at <= ? AND id > ? ORDER BY id ASC LIMIT ?`, now, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.APIRoute{}
	for rows.Next() {
		var rt models.APIRoute
		if err = scanRoute(rows, &rt); err != nil {
			return nil, err
		}
		out = append(out, rt)
	}
	return out, rows.Err()
}

func (r *Store) MarkRouteChecked(ctx context.Context, id int64, status string, statusCode, latencyMS int, failureReason string, consecutiveFailures, consecutiveSuccesses int, routeStatus string, checkedAt, nextCheckAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_routes SET last_checked_at=?, last_status_code=?, last_latency_ms=?, last_failure_reason=?,
			consecutive_failures=?, consecutive_successes=?, status=?, next_check_at=?, updated_at=NOW()
		WHERE id=?`,
		checkedAt, statusCode, latencyMS, nullableString(failureReason), consecutiveFailures, consecutiveSuccesses, routeStatus, nextCheckAt, id)
	return err
}

func (r *Store) RecordRouteCheck(ctx context.Context, check models.RouteCheck) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO route_checks (route_id, project_id, status, status_code, latency_ms, failure_reason, attempt, checked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		check.RouteID, check.ProjectID, check.Status, check.StatusCode, check.LatencyMS, nullableString(check.FailureReason), check.Attempt, check.CheckedAt)
	return err
}

func (r *Store) ListRouteChecks(ctx context.Context, routeID int64, limit, offset int) ([]models.RouteCheck, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, route_id, project_id, status, status_code, latency_ms, failure_reason, attempt, checked_at, created_at
		FROM route_checks WHERE route_id=? ORDER BY checked_at DESC LIMIT ? OFFSET ?`, routeID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.RouteCheck{}
	for rows.Next() {
		var c models.RouteCheck
		var reason sql.NullString
		if err = rows.Scan(&c.ID, &c.RouteID, &c.ProjectID, &c.Status, &c.StatusCode, &c.LatencyMS, &reason, &c.Attempt, &c.CheckedAt, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.FailureReason = reason.String
		out = append(out, c)
	}
	return out, rows.Err()
}

// AggregateCheckTimeseries buckets raw checks into fixed-width intervals in a
// single grouped query. The (project_id, checked_at) and (route_id, checked_at)
// indexes cover the WHERE clause, and maxBuckets caps the row count so a wide
// requested range can never produce an unbounded response.
func (r *Store) AggregateCheckTimeseries(ctx context.Context, projectID int64, routeID *int64, since time.Time, bucketSeconds, maxBuckets int) ([]models.MetricPoint, error) {
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	if maxBuckets <= 0 || maxBuckets > 2000 {
		maxBuckets = 500
	}
	query := `SELECT FROM_UNIXTIME(FLOOR(UNIX_TIMESTAMP(checked_at) / ?) * ?) AS bucket_start,
			COUNT(*) AS checks,
			COALESCE(SUM(status = 'down'), 0) AS failures,
			COALESCE(ROUND(AVG(latency_ms)), 0) AS avg_latency,
			COALESCE(MAX(latency_ms), 0) AS max_latency
		FROM route_checks
		WHERE project_id = ? AND checked_at >= ?`
	args := []any{bucketSeconds, bucketSeconds, projectID, since}
	if routeID != nil {
		query += ` AND route_id = ?`
		args = append(args, *routeID)
	}
	query += ` GROUP BY bucket_start ORDER BY bucket_start ASC LIMIT ?`
	args = append(args, maxBuckets)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.MetricPoint{}
	for rows.Next() {
		var p models.MetricPoint
		if err = rows.Scan(&p.BucketStart, &p.Checks, &p.Failures, &p.AvgLatencyMS, &p.MaxLatencyMS); err != nil {
			return nil, err
		}
		if p.Checks > 0 {
			p.UptimePct = float64(p.Checks-p.Failures) / float64(p.Checks) * 100
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AggregateRouteMetrics recomputes the 24h rolling window columns for every
// route in a single batched UPDATE...JOIN, avoiding N+1 per-route queries.
func (r *Store) AggregateRouteMetrics(ctx context.Context, since time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE api_routes r
		LEFT JOIN (
			SELECT route_id,
				COUNT(*) AS checks,
				SUM(CASE WHEN status='down' THEN 1 ELSE 0 END) AS failures,
				AVG(latency_ms) AS avg_latency,
				SUM(CASE WHEN status='up' THEN 1 ELSE 0 END) AS successes
			FROM route_checks
			WHERE checked_at >= ?
			GROUP BY route_id
		) agg ON agg.route_id = r.id
		SET r.checks_24h = COALESCE(agg.checks, 0),
			r.failures_24h = COALESCE(agg.failures, 0),
			r.avg_latency_24h_ms = COALESCE(ROUND(agg.avg_latency), 0),
			r.uptime_24h_pct = CASE WHEN COALESCE(agg.checks, 0) = 0 THEN 0 ELSE ROUND(100 * COALESCE(agg.successes, 0) / agg.checks, 2) END
	`, since)
	return err
}

// AggregateProjectMetrics recomputes cached per-project dashboard counters
// from the (already aggregated) route rows and open incident counts.
func (r *Store) AggregateProjectMetrics(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE projects p
		LEFT JOIN (
			SELECT project_id,
				COUNT(*) AS total,
				SUM(CASE WHEN status='healthy' THEN 1 ELSE 0 END) AS healthy,
				SUM(CASE WHEN status='degraded' THEN 1 ELSE 0 END) AS degraded,
				SUM(CASE WHEN status='failing' THEN 1 ELSE 0 END) AS failing,
				SUM(CASE WHEN status='disabled' THEN 1 ELSE 0 END) AS disabled,
				SUM(CASE WHEN status='unknown' THEN 1 ELSE 0 END) AS unknown,
				SUM(checks_24h) AS checks,
				SUM(failures_24h) AS failures,
				AVG(NULLIF(avg_latency_24h_ms, 0)) AS avg_latency,
				MAX(last_checked_at) AS last_check
			FROM api_routes
			GROUP BY project_id
		) r ON r.project_id = p.id
		LEFT JOIN (
			SELECT project_id, COUNT(*) AS open_count FROM route_incidents WHERE state='open' GROUP BY project_id
		) i ON i.project_id = p.id
		SET p.routes_total = COALESCE(r.total, 0),
			p.routes_healthy = COALESCE(r.healthy, 0),
			p.routes_degraded = COALESCE(r.degraded, 0),
			p.routes_failing = COALESCE(r.failing, 0),
			p.routes_disabled = COALESCE(r.disabled, 0),
			p.routes_unknown = COALESCE(r.unknown, 0),
			p.checks_24h = COALESCE(r.checks, 0),
			p.failures_24h = COALESCE(r.failures, 0),
			p.avg_latency_24h_ms = COALESCE(ROUND(r.avg_latency), 0),
			p.uptime_24h_pct = CASE WHEN COALESCE(r.checks, 0) = 0 THEN 0 ELSE ROUND(100 * (r.checks - r.failures) / r.checks, 2) END,
			p.open_incidents = COALESCE(i.open_count, 0),
			p.last_check_at = r.last_check,
			p.metrics_updated_at = UTC_TIMESTAMP()
	`)
	return err
}

// PruneRouteChecks deletes aged-out time-series rows in bounded batches so a
// single retention pass never holds a long-running transaction/lock.
func (r *Store) PruneRouteChecks(ctx context.Context, before time.Time, batchSize int) (int64, error) {
	if batchSize <= 0 || batchSize > 10000 {
		batchSize = 5000
	}
	var totalDeleted int64
	for {
		res, err := r.db.ExecContext(ctx, `DELETE FROM route_checks WHERE checked_at < ? LIMIT ?`, before, batchSize)
		if err != nil {
			return totalDeleted, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return totalDeleted, err
		}
		totalDeleted += n
		if n < int64(batchSize) {
			break
		}
	}
	return totalDeleted, nil
}
