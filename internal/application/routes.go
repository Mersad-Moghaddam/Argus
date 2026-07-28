package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
)

type RouteInput struct {
	Method              string
	Path                string
	BaseURL             string
	Name                string
	Summary             string
	Description         string
	Tags                []string
	Deprecated          bool
	Headers             string
	Enabled             *bool
	MonitorIntervalSecs int
	TimeoutMS           int
	Retries             int
	ExpectedStatusRange string
	FailureThreshold    int
	RecoverySuccesses   int
}

func (s *Service) CreateRoute(ctx context.Context, project models.Project, input RouteInput) (models.APIRoute, error) {
	route, err := s.buildRoute(project, input, "manual")
	if err != nil {
		return models.APIRoute{}, err
	}
	existing, err := s.routes.GetRouteByMethodPath(ctx, project.ID, route.Method, route.Path)
	if err != nil {
		return models.APIRoute{}, err
	}
	if existing != nil {
		return models.APIRoute{}, domain.ErrDuplicateRoute
	}
	id, err := s.routes.CreateRoute(ctx, route)
	if err != nil {
		return models.APIRoute{}, err
	}
	route.ID = id
	return route, nil
}

// BulkCreateResult reports a per-row outcome for a bulk manual add, so
// partial failures never silently drop rows.
type BulkCreateResult struct {
	Created []models.APIRoute `json:"created"`
	Failed  []BulkCreateError `json:"failed"`
}
type BulkCreateError struct {
	Index int    `json:"index"`
	Route string `json:"route"`
	Error string `json:"error"`
}

func (s *Service) BulkCreateRoutes(ctx context.Context, project models.Project, inputs []RouteInput) (BulkCreateResult, error) {
	result := BulkCreateResult{Created: []models.APIRoute{}, Failed: []BulkCreateError{}}
	existing, err := s.routes.ListAllRouteKeys(ctx, project.ID)
	if err != nil {
		return result, err
	}
	seen := map[string]bool{}
	toInsert := []models.APIRoute{}
	pending := []RouteInput{}
	for i, in := range inputs {
		route, buildErr := s.buildRoute(project, in, "manual")
		label := strings.TrimSpace(in.Method + " " + in.Path)
		if buildErr != nil {
			result.Failed = append(result.Failed, BulkCreateError{Index: i, Route: label, Error: buildErr.Error()})
			continue
		}
		key := route.Method + " " + route.Path
		if existing[key] != 0 || seen[key] {
			result.Failed = append(result.Failed, BulkCreateError{Index: i, Route: key, Error: domain.ErrDuplicateRoute.Error()})
			continue
		}
		seen[key] = true
		toInsert = append(toInsert, route)
		pending = append(pending, in)
	}
	if len(toInsert) > 0 {
		if _, err = s.routes.BulkCreateRoutes(ctx, toInsert); err != nil {
			return result, err
		}
		for _, route := range toInsert {
			created, lookupErr := s.routes.GetRouteByMethodPath(ctx, project.ID, route.Method, route.Path)
			if lookupErr == nil && created != nil {
				result.Created = append(result.Created, *created)
			}
		}
	}
	_ = pending
	return result, nil
}

func (s *Service) buildRoute(project models.Project, input RouteInput, source string) (models.APIRoute, error) {
	normalized, err := domain.NormalizeEndpoint(input.Method, input.BaseURL, input.Path)
	if err != nil {
		return models.APIRoute{}, err
	}
	method, path := normalized.Method, normalized.RouteTemplate
	// Compatibility callers without an explicit Enabled field retain their
	// legacy behavior. All v2 browser and import paths send an explicit value;
	// import is catalog-only and browser defaults are disabled.
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	if input.Enabled != nil && enabled && !domain.IsSafeSyntheticMethod(method) {
		return models.APIRoute{}, domain.ErrUnsafeSynthetic
	}
	interval := orDefault(input.MonitorIntervalSecs, project.DefaultIntervalSeconds, 10, 86400)
	timeout := orDefault(input.TimeoutMS, project.DefaultTimeoutMS, 200, 60000)
	retries := orDefault(input.Retries, project.DefaultRetries, 0, 5)
	failureThreshold := orDefault(input.FailureThreshold, project.FailureThreshold, 1, 20)
	recoverySuccesses := orDefault(input.RecoverySuccesses, project.RecoverySuccessThreshold, 1, 20)
	expectedRange := strings.TrimSpace(input.ExpectedStatusRange)
	if expectedRange == "" {
		expectedRange = "200-399"
	}
	status := domain.ComputeRouteStatus(domain.RouteHealthInput{Enabled: enabled, Checked: false})

	route := models.APIRoute{
		ProjectID:           project.ID,
		Method:              method,
		Path:                path,
		BaseURL:             normalized.BaseURL,
		Name:                strings.TrimSpace(input.Name),
		Summary:             strings.TrimSpace(input.Summary),
		Description:         strings.TrimSpace(input.Description),
		Tags:                input.Tags,
		Deprecated:          input.Deprecated,
		Headers:             redactUnsafeHeaderKeysNoop(input.Headers),
		Source:              source,
		Enabled:             enabled,
		MonitorIntervalSecs: interval,
		TimeoutMS:           timeout,
		Retries:             retries,
		ExpectedStatusRange: expectedRange,
		FailureThreshold:    failureThreshold,
		RecoverySuccesses:   recoverySuccesses,
		Status:              status,
		NextCheckAt:         time.Now().UTC(),
	}
	return route, nil
}

// redactUnsafeHeaderKeysNoop validates the headers value is well-formed JSON
// (or empty); actual secret redaction happens on read via RedactHeaders.
func redactUnsafeHeaderKeysNoop(headers string) string {
	headers = strings.TrimSpace(headers)
	if headers == "" {
		return ""
	}
	var probe map[string]string
	if err := json.Unmarshal([]byte(headers), &probe); err != nil {
		return ""
	}
	return headers
}

var sensitiveHeaderNames = map[string]bool{
	"authorization": true, "x-api-key": true, "api-key": true, "cookie": true, "set-cookie": true,
	"x-auth-token": true, "proxy-authorization": true,
}

// RedactHeaders masks sensitive header values before they are ever returned
// to a client, so secrets configured for route checks are never echoed back
// in API responses, import previews, or logs.
func RedactHeaders(headersJSON string) string {
	if strings.TrimSpace(headersJSON) == "" {
		return ""
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
		return ""
	}
	for k := range headers {
		if sensitiveHeaderNames[strings.ToLower(k)] {
			headers[k] = "••••••••"
		}
	}
	out, err := json.Marshal(headers)
	if err != nil {
		return ""
	}
	return string(out)
}

func (s *Service) UpdateRoute(ctx context.Context, existing models.APIRoute, input RouteInput) (models.APIRoute, error) {
	if input.Name != "" {
		existing.Name = strings.TrimSpace(input.Name)
	}
	if input.Summary != "" {
		existing.Summary = strings.TrimSpace(input.Summary)
	}
	if input.Description != "" {
		existing.Description = strings.TrimSpace(input.Description)
	}
	if input.Tags != nil {
		existing.Tags = input.Tags
	}
	existing.Deprecated = input.Deprecated
	if input.BaseURL != "" {
		baseURL, _, err := domain.NormalizeBaseURL(input.BaseURL)
		if err != nil {
			return models.APIRoute{}, err
		}
		existing.BaseURL = baseURL
	}
	if raw := redactUnsafeHeaderKeysNoop(input.Headers); raw != "" || input.Headers == "" {
		existing.Headers = raw
	}
	if input.Enabled != nil {
		existing.Enabled = *input.Enabled
	}
	existing.MonitorIntervalSecs = orDefault(input.MonitorIntervalSecs, existing.MonitorIntervalSecs, 10, 86400)
	existing.TimeoutMS = orDefault(input.TimeoutMS, existing.TimeoutMS, 200, 60000)
	existing.Retries = orDefault(input.Retries, existing.Retries, 0, 5)
	existing.FailureThreshold = orDefault(input.FailureThreshold, existing.FailureThreshold, 1, 20)
	existing.RecoverySuccesses = orDefault(input.RecoverySuccesses, existing.RecoverySuccesses, 1, 20)
	if r := strings.TrimSpace(input.ExpectedStatusRange); r != "" {
		existing.ExpectedStatusRange = r
	}
	existing.Status = domain.ComputeRouteStatus(domain.RouteHealthInput{
		Enabled: existing.Enabled, Checked: existing.LastCheckedAt != nil,
		LastStatus:          lastStatusLabel(existing.LastStatusCode, existing.LastFailureReason),
		ConsecutiveFailures: existing.ConsecutiveFailures, ConsecutiveSuccesses: existing.ConsecutiveSuccesses,
		FailureThreshold: existing.FailureThreshold,
	})
	if err := s.routes.UpdateRoute(ctx, existing); err != nil {
		return models.APIRoute{}, err
	}
	return existing, nil
}

func lastStatusLabel(statusCode int, failureReason string) string {
	if failureReason != "" {
		return "down"
	}
	if statusCode >= 200 && statusCode < 400 {
		return "up"
	}
	return "down"
}

func (s *Service) SetRouteEnabled(ctx context.Context, id int64, enabled bool) error {
	if enabled {
		route, err := s.routes.GetRouteByID(ctx, id)
		if err != nil {
			return err
		}
		if route == nil {
			return domain.ErrRouteNotFound
		}
		if !domain.IsSafeSyntheticMethod(route.Method) {
			return domain.ErrUnsafeSynthetic
		}
	}
	return s.routes.SetRouteEnabled(ctx, id, enabled)
}

func (s *Service) DeleteRoute(ctx context.Context, id int64) error {
	return s.routes.DeleteRoute(ctx, id)
}

func (s *Service) BulkDeleteRoutes(ctx context.Context, projectID int64, ids []int64) (int64, error) {
	return s.routes.BulkDeleteRoutes(ctx, projectID, ids)
}

func (s *Service) GetRoute(ctx context.Context, id int64) (*models.APIRoute, error) {
	return s.routes.GetRouteByID(ctx, id)
}

func (s *Service) GetRouteByMethodPath(ctx context.Context, projectID int64, method, path string) (*models.APIRoute, error) {
	return s.routes.GetRouteByMethodPath(ctx, projectID, method, path)
}

func (s *Service) ListRoutes(ctx context.Context, filter models.RouteFilter) ([]models.APIRoute, int, error) {
	return s.routes.ListRoutes(ctx, filter)
}

func (s *Service) ListRouteChecks(ctx context.Context, routeID int64, limit, offset int) ([]models.RouteCheck, error) {
	return s.routes.ListRouteChecks(ctx, routeID, limit, offset)
}

func (s *Service) ListRouteIncidents(ctx context.Context, projectID int64, routeID *int64, state string, limit, offset int) ([]models.RouteIncident, error) {
	return s.routeIncidents.ListRouteIncidents(ctx, projectID, routeID, state, limit, offset)
}

// timeseriesRanges are the only windows the API will serve. Fixing the set
// (rather than accepting an arbitrary since/bucket pair) keeps every chart
// query bounded and index-friendly, and keeps the bucket count small enough
// to render without a charting library.
var timeseriesRanges = map[string]models.TimeseriesWindow{
	"1h":  {Range: "1h", BucketSeconds: 120},    // 30 buckets
	"6h":  {Range: "6h", BucketSeconds: 600},    // 36 buckets
	"24h": {Range: "24h", BucketSeconds: 1800},  // 48 buckets
	"7d":  {Range: "7d", BucketSeconds: 10800},  // 56 buckets
	"30d": {Range: "30d", BucketSeconds: 86400}, // 30 buckets
}

var timeseriesSpans = map[string]time.Duration{
	"1h": time.Hour, "6h": 6 * time.Hour, "24h": 24 * time.Hour,
	"7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour,
}

// DefaultTimeseriesRange is used when a caller omits or misspells the range.
const DefaultTimeseriesRange = "24h"

// maxTimeseriesBuckets bounds the response regardless of the requested range.
const maxTimeseriesBuckets = 500

// MetricsTimeseries is a bounded, bucketed view of a project's (or a single
// route's) check history, ready for charting.
type MetricsTimeseries struct {
	models.TimeseriesWindow
	Points []models.MetricPoint `json:"points"`
}

// ListMetricsTimeseries resolves a named range into a bounded bucketed query.
// Unknown ranges fall back to the default rather than erroring, so a stale
// bookmark cannot break the dashboard.
func (s *Service) ListMetricsTimeseries(ctx context.Context, projectID int64, routeID *int64, rangeKey string) (MetricsTimeseries, error) {
	window, ok := timeseriesRanges[strings.ToLower(strings.TrimSpace(rangeKey))]
	if !ok {
		window = timeseriesRanges[DefaultTimeseriesRange]
	}
	window.Since = time.Now().UTC().Add(-timeseriesSpans[window.Range])

	points, err := s.routes.AggregateCheckTimeseries(ctx, projectID, routeID, window.Since, window.BucketSeconds, maxTimeseriesBuckets)
	if err != nil {
		return MetricsTimeseries{}, err
	}
	return MetricsTimeseries{TimeseriesWindow: window, Points: points}, nil
}

// ProcessRouteCheckResult is the single place that turns a raw check
// outcome into updated route health state, a persisted check record, and
// (if warranted) an incident open/resolve transition. It is the route
// analogue of ProcessIncidentTransition for website monitors.
func (s *Service) ProcessRouteCheckResult(ctx context.Context, route models.APIRoute, status string, statusCode, latencyMS int, failureReason string, attempts int, checkedAt time.Time) error {
	consecutiveFailures := route.ConsecutiveFailures
	consecutiveSuccesses := route.ConsecutiveSuccesses
	if status == "up" {
		consecutiveFailures = 0
		consecutiveSuccesses++
	} else {
		consecutiveFailures++
		consecutiveSuccesses = 0
	}

	newStatus := domain.ComputeRouteStatus(domain.RouteHealthInput{
		Enabled: route.Enabled, Checked: true, LastStatus: status,
		ConsecutiveFailures: consecutiveFailures, ConsecutiveSuccesses: consecutiveSuccesses,
		FailureThreshold: route.FailureThreshold,
	})
	nextCheckAt := checkedAt.Add(time.Duration(route.MonitorIntervalSecs) * time.Second)

	if err := s.routes.MarkRouteChecked(ctx, route.ID, status, statusCode, latencyMS, failureReason, consecutiveFailures, consecutiveSuccesses, newStatus, checkedAt, nextCheckAt); err != nil {
		return err
	}
	if err := s.routes.RecordRouteCheck(ctx, models.RouteCheck{
		RouteID: route.ID, ProjectID: route.ProjectID, Status: status, StatusCode: statusCode,
		LatencyMS: latencyMS, FailureReason: failureReason, Attempt: attempts, CheckedAt: checkedAt,
	}); err != nil {
		return err
	}

	openIncident, err := s.routeIncidents.GetOpenRouteIncident(ctx, route.ID)
	if err != nil {
		return err
	}
	transition := domain.RouteIncidentPolicy(openIncident != nil, consecutiveFailures, route.FailureThreshold, consecutiveSuccesses, route.RecoverySuccesses)
	if !transition.ShouldOpen && !transition.ShouldResolve {
		return nil
	}

	bucket := checkedAt.UTC().Truncate(time.Minute).Format(time.RFC3339)
	if transition.ShouldOpen {
		incidentID, createErr := s.routeIncidents.CreateRouteIncident(ctx, route.ID, route.ProjectID, failureReason, checkedAt)
		if createErr != nil {
			return createErr
		}
		payload, _ := json.Marshal(OutboxPayload{Event: "route_incident_opened", WebsiteID: route.ID, IncidentID: incidentID, URL: route.Method + " " + route.Path, Message: failureReason, Timestamp: checkedAt.Format(time.RFC3339)})
		return s.outbox.AddEvent(ctx, "route_incident_opened", incidentID, fmt.Sprintf("route:%d:incident_opened:%s", route.ID, bucket), payload, checkedAt)
	}

	if openIncident == nil {
		return nil
	}
	if err = s.routeIncidents.ResolveRouteIncident(ctx, openIncident.ID, checkedAt); err != nil {
		return err
	}
	payload, _ := json.Marshal(OutboxPayload{Event: "route_incident_resolved", WebsiteID: route.ID, IncidentID: openIncident.ID, URL: route.Method + " " + route.Path, Message: "route recovered", Timestamp: checkedAt.Format(time.RFC3339)})
	return s.outbox.AddEvent(ctx, "route_incident_resolved", openIncident.ID, fmt.Sprintf("route:%d:incident_resolved:%s", route.ID, bucket), payload, checkedAt)
}
