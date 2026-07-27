package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
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

var ErrInvalidBaseURL = errors.New("base URL must be a public HTTP or HTTPS URL")

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
	method, err := domain.NormalizeMethod(input.Method)
	if err != nil {
		return models.APIRoute{}, err
	}
	path, err := domain.NormalizePath(input.Path)
	if err != nil {
		return models.APIRoute{}, err
	}
	baseURL, err := validateBaseURL(input.BaseURL)
	if err != nil {
		return models.APIRoute{}, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
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
	headers, err := normalizeHeaders(input.Headers)
	if err != nil {
		return models.APIRoute{}, err
	}

	route := models.APIRoute{
		ProjectID:           project.ID,
		Method:              method,
		Path:                path,
		BaseURL:             baseURL,
		Name:                strings.TrimSpace(input.Name),
		Summary:             strings.TrimSpace(input.Summary),
		Description:         strings.TrimSpace(input.Description),
		Tags:                input.Tags,
		Deprecated:          input.Deprecated,
		Headers:             headers,
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

func validateBaseURL(raw string) (string, error) {
	normalized := strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(normalized)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return "", ErrInvalidBaseURL
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	if host == "localhost" || host == "metadata.google.internal" || strings.HasSuffix(host, ".metadata.google.internal") {
		return "", ErrInvalidBaseURL
	}
	if address, parseErr := netip.ParseAddr(host); parseErr == nil {
		address = address.Unmap()
		if !address.IsValid() || address.IsUnspecified() || address.IsLoopback() || address.IsPrivate() ||
			address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() {
			return "", ErrInvalidBaseURL
		}
	}
	return normalized, nil
}

func normalizeHeaders(headers string) (string, error) {
	headers = strings.TrimSpace(headers)
	if headers == "" {
		return "", nil
	}
	var probe map[string]string
	if err := json.Unmarshal([]byte(headers), &probe); err != nil {
		return "", domain.ErrInvalidInput
	}
	normalized, err := json.Marshal(probe)
	if err != nil {
		return "", domain.ErrInvalidInput
	}
	return string(normalized), nil
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
		baseURL, err := validateBaseURL(input.BaseURL)
		if err != nil {
			return models.APIRoute{}, err
		}
		existing.BaseURL = baseURL
	}
	// A route read deliberately returns redacted headers. Treat an omitted or
	// blank edit value as "preserve" so an unrelated configuration edit cannot
	// silently erase stored credentials. Callers can explicitly clear headers
	// by sending an empty JSON object.
	if strings.TrimSpace(input.Headers) != "" {
		headers, err := normalizeHeaders(input.Headers)
		if err != nil {
			return models.APIRoute{}, err
		}
		existing.Headers = headers
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

func (s *Service) ListRoutes(ctx context.Context, filter models.RouteFilter) ([]models.APIRoute, int, error) {
	return s.routes.ListRoutes(ctx, filter)
}

func (s *Service) ListRouteChecks(ctx context.Context, routeID int64, limit, offset int) ([]models.RouteCheck, error) {
	return s.routes.ListRouteChecks(ctx, routeID, limit, offset)
}

func (s *Service) ListProjectMetricSeries(ctx context.Context, projectID int64, since time.Time, bucketSeconds int) ([]models.ProjectMetricPoint, error) {
	return s.routes.ListProjectMetricSeries(ctx, projectID, since, bucketSeconds)
}

func (s *Service) ListRouteIncidents(ctx context.Context, projectID int64, routeID *int64, state string, limit, offset int) ([]models.RouteIncident, error) {
	return s.routeIncidents.ListRouteIncidents(ctx, projectID, routeID, state, limit, offset)
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
		if openIncident != nil && status != "up" {
			return s.routeIncidents.RecordRouteIncidentFailure(ctx, openIncident.ID, failureReason)
		}
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
