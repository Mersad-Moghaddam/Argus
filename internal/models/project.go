package models

import "time"

// Project groups a set of monitored API routes.
type Project struct {
	ID                       int64     `json:"id"`
	OwnerUserID              int64     `json:"ownerUserId"`
	Name                     string    `json:"name"`
	Slug                     string    `json:"slug"`
	Description              string    `json:"description,omitempty"`
	Status                   string    `json:"status"`
	DefaultIntervalSeconds   int       `json:"defaultIntervalSeconds"`
	DefaultTimeoutMS         int       `json:"defaultTimeoutMs"`
	DefaultRetries           int       `json:"defaultRetries"`
	FailureThreshold         int       `json:"failureThreshold"`
	RecoverySuccessThreshold int       `json:"recoverySuccessThreshold"`
	CreatedAt                time.Time `json:"createdAt"`
	UpdatedAt                time.Time `json:"updatedAt"`

	// Aggregated, periodically refreshed metrics (see route:aggregate_metrics job).
	RoutesTotal      int        `json:"routesTotal"`
	RoutesHealthy    int        `json:"routesHealthy"`
	RoutesDegraded   int        `json:"routesDegraded"`
	RoutesFailing    int        `json:"routesFailing"`
	RoutesDisabled   int        `json:"routesDisabled"`
	RoutesUnknown    int        `json:"routesUnknown"`
	Uptime24hPct     float64    `json:"uptime24hPct"`
	AvgLatency24hMS  int        `json:"avgLatency24hMs"`
	Checks24h        int        `json:"checks24h"`
	Failures24h      int        `json:"failures24h"`
	OpenIncidents    int        `json:"openIncidents"`
	LastCheckAt      *time.Time `json:"lastCheckAt,omitempty"`
	MetricsUpdatedAt *time.Time `json:"metricsUpdatedAt,omitempty"`

	// Role of the requesting user, populated by handlers, not persisted here.
	ViewerRole string `json:"viewerRole,omitempty"`
}

// ProjectFilter narrows a project listing.
type ProjectFilter struct {
	Search string
	Status string
	Limit  int
	Offset int
}
