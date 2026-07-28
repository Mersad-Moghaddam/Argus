package domain

import (
	"errors"
	"strings"
)

// Route health states. These are the only valid values surfaced to clients.
const (
	RouteStatusHealthy  = "healthy"
	RouteStatusDegraded = "degraded"
	RouteStatusFailing  = "failing"
	RouteStatusDisabled = "disabled"
	RouteStatusUnknown  = "unknown"
)

const (
	ProjectStatusActive   = "active"
	ProjectStatusArchived = "archived"
)

var (
	ErrProjectNotFound    = errors.New("project not found")
	ErrProjectForbidden   = errors.New("project access forbidden")
	ErrRouteNotFound      = errors.New("route not found")
	ErrDuplicateRoute     = errors.New("route already exists for method and path")
	ErrInvalidRoute       = errors.New("invalid route definition")
	ErrImportJobNotFound  = errors.New("import job not found")
	ErrImportJobCommitted = errors.New("import job already committed")
	ErrUnsafeSynthetic    = errors.New("only GET and HEAD can be enabled as synthetic checks")
)

// DefaultFailureThreshold is the number of consecutive failures required to
// transition a route into the "failing" state and open an incident.
const DefaultFailureThreshold = 3

// DefaultRecoverySuccesses is the number of consecutive successes required to
// resolve an open incident and return a route to "healthy".
const DefaultRecoverySuccesses = 1

// RouteHealthInput captures the minimal state needed to derive a route's
// externally visible status. Kept as pure data so the policy below is a pure
// function that is trivial to unit test.
type RouteHealthInput struct {
	Enabled              bool
	Checked              bool
	LastStatus           string // "up" or "down"
	ConsecutiveFailures  int
	ConsecutiveSuccesses int
	FailureThreshold     int
}

// IsSafeSyntheticMethod is deliberately narrower than the HTTP safe-method
// definition. OPTIONS can still have target-specific side effects, so the
// product starts with only the predictable read-only canary methods.
func IsSafeSyntheticMethod(method string) bool { return method == "GET" || method == "HEAD" }

// ComputeRouteStatus is the single source of truth for route health state.
func ComputeRouteStatus(in RouteHealthInput) string {
	if !in.Enabled {
		return RouteStatusDisabled
	}
	if !in.Checked {
		return RouteStatusUnknown
	}
	threshold := in.FailureThreshold
	if threshold <= 0 {
		threshold = DefaultFailureThreshold
	}
	if in.LastStatus == "up" && in.ConsecutiveFailures == 0 {
		return RouteStatusHealthy
	}
	if in.ConsecutiveFailures >= threshold {
		return RouteStatusFailing
	}
	if in.ConsecutiveFailures > 0 {
		return RouteStatusDegraded
	}
	return RouteStatusHealthy
}

// RouteIncidentPolicy decides whether a check result should open or resolve
// a route incident. It is intentionally symmetric with IncidentPolicy used
// for website monitors so both subsystems share the same mental model.
func RouteIncidentPolicy(currentOpen bool, consecutiveFailures, failureThreshold, consecutiveSuccesses, recoverySuccesses int) IncidentTransition {
	if failureThreshold <= 0 {
		failureThreshold = DefaultFailureThreshold
	}
	if recoverySuccesses <= 0 {
		recoverySuccesses = DefaultRecoverySuccesses
	}
	if !currentOpen && consecutiveFailures >= failureThreshold {
		return IncidentTransition{ShouldOpen: true}
	}
	if currentOpen && consecutiveSuccesses >= recoverySuccesses {
		return IncidentTransition{ShouldResolve: true}
	}
	return IncidentTransition{}
}

// NormalizeMethod upper-cases and validates an HTTP method against the set
// supported by OpenAPI/Swagger operations.
func NormalizeMethod(method string) (string, error) {
	m := strings.ToUpper(strings.TrimSpace(method))
	switch m {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE":
		return m, nil
	default:
		return "", ErrInvalidRoute
	}
}

// NormalizePath ensures a route path is stored consistently (leading slash,
// no trailing slash unless root, no surrounding whitespace).
func NormalizePath(path string) (string, error) {
	p := strings.TrimSpace(path)
	if p == "" {
		return "", ErrInvalidRoute
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 && strings.HasSuffix(p, "/") {
		p = strings.TrimRight(p, "/")
	}
	return p, nil
}
