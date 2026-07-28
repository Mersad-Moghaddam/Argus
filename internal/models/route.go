package models

import "time"

// APIRoute represents a single monitored operation (method+path) inside a project.
type APIRoute struct {
	ID                   int64      `json:"id"`
	ProjectID            int64      `json:"projectId"`
	Method               string     `json:"method"`
	Path                 string     `json:"path"`
	BaseURL              string     `json:"baseUrl"`
	CanonicalIdentity    string     `json:"canonicalIdentity,omitempty"`
	CanonicalHash        []byte     `json:"-"`
	CanonicalVersion     int        `json:"canonicalVersion,omitempty"`
	OperationID          string     `json:"operationId,omitempty"`
	Name                 string     `json:"name,omitempty"`
	Summary              string     `json:"summary,omitempty"`
	Description          string     `json:"description,omitempty"`
	Tags                 []string   `json:"tags,omitempty"`
	Deprecated           bool       `json:"deprecated"`
	Parameters           string     `json:"parameters,omitempty"`  // raw JSON
	RequestBody          string     `json:"requestBody,omitempty"` // raw JSON
	Responses            string     `json:"responses,omitempty"`   // raw JSON
	Security             string     `json:"security,omitempty"`    // raw JSON
	Headers              string     `json:"headers,omitempty"`     // raw JSON, redacted values are masked on read
	SpecHash             string     `json:"specHash,omitempty"`
	Source               string     `json:"source"` // manual | import
	Enabled              bool       `json:"enabled"`
	MonitorIntervalSecs  int        `json:"monitorIntervalSeconds"`
	TimeoutMS            int        `json:"timeoutMs"`
	Retries              int        `json:"retries"`
	ExpectedStatusRange  string     `json:"expectedStatusRange"`
	FailureThreshold     int        `json:"failureThreshold"`
	RecoverySuccesses    int        `json:"recoverySuccesses"`
	Status               string     `json:"status"`
	LastCheckedAt        *time.Time `json:"lastCheckedAt,omitempty"`
	LastStatusCode       int        `json:"lastStatusCode"`
	LastLatencyMS        int        `json:"lastLatencyMs"`
	LastFailureReason    string     `json:"lastFailureReason,omitempty"`
	ConsecutiveFailures  int        `json:"consecutiveFailures"`
	ConsecutiveSuccesses int        `json:"consecutiveSuccesses"`
	NextCheckAt          time.Time  `json:"nextCheckAt"`
	Uptime24hPct         float64    `json:"uptime24hPct"`
	AvgLatency24hMS      int        `json:"avgLatency24hMs"`
	Checks24h            int        `json:"checks24h"`
	Failures24h          int        `json:"failures24h"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

// RouteFilter narrows a route listing/search within a project.
type RouteFilter struct {
	ProjectID  int64
	Search     string
	Method     string
	Status     string
	Tag        string
	Enabled    *bool
	Deprecated *bool
	SortBy     string // path|method|status|latency|uptime|updated
	SortDir    string // asc|desc
	Limit      int
	Offset     int
}

// RouteCheck is one time-series data point recorded for a route.
type RouteCheck struct {
	ID            int64     `json:"id"`
	RouteID       int64     `json:"routeId"`
	ProjectID     int64     `json:"projectId"`
	Status        string    `json:"status"`
	StatusCode    int       `json:"statusCode"`
	LatencyMS     int       `json:"latencyMs"`
	FailureReason string    `json:"failureReason,omitempty"`
	Attempt       int       `json:"attempt"`
	CheckedAt     time.Time `json:"checkedAt"`
	CreatedAt     time.Time `json:"createdAt"`
}

// MetricPoint is one aggregated time bucket of check results, used to draw
// the dashboard's uptime and latency charts without shipping raw check rows
// to the browser.
type MetricPoint struct {
	BucketStart  time.Time `json:"bucketStart"`
	Checks       int       `json:"checks"`
	Failures     int       `json:"failures"`
	UptimePct    float64   `json:"uptimePct"`
	AvgLatencyMS int       `json:"avgLatencyMs"`
	MaxLatencyMS int       `json:"maxLatencyMs"`
}

// TimeseriesWindow describes a bounded, bucketed query over route_checks.
type TimeseriesWindow struct {
	Range         string    `json:"range"`
	Since         time.Time `json:"since"`
	BucketSeconds int       `json:"bucketSeconds"`
}

// RouteIncident tracks an open/resolved failure window for a route.
type RouteIncident struct {
	ID                int64      `json:"id"`
	RouteID           int64      `json:"routeId"`
	ProjectID         int64      `json:"projectId"`
	State             string     `json:"state"` // open | resolved
	StartedAt         time.Time  `json:"startedAt"`
	ResolvedAt        *time.Time `json:"resolvedAt,omitempty"`
	FailureCount      int        `json:"failureCount"`
	LastFailureReason string     `json:"lastFailureReason,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}
