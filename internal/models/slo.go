package models

import "time"

// SLODefinition is the current, project-scoped policy. Its Version is copied
// into every evaluation so a later policy change cannot rewrite history.
type SLODefinition struct {
	ID                 int64     `json:"id"`
	ProjectID          int64     `json:"projectId"`
	CreatedByUserID    int64     `json:"createdByUserId"`
	Name               string    `json:"name"`
	SLIKind            string    `json:"sliKind"`
	TargetPercent      float64   `json:"targetPercent"`
	WindowSeconds      int       `json:"windowSeconds"`
	LatencyThresholdMS int       `json:"latencyThresholdMs,omitempty"`
	MinEvents          int       `json:"minEvents"`
	ShortWindowSeconds int       `json:"shortWindowSeconds"`
	ShortBurnRate      float64   `json:"shortBurnRate"`
	LongWindowSeconds  int       `json:"longWindowSeconds"`
	LongBurnRate       float64   `json:"longBurnRate"`
	Paused             bool      `json:"paused"`
	Version            int       `json:"version"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// SLOEvaluation is a compact, auditable result. It has no raw request, URL,
// trace, or metric-label payloads, which keeps the control plane bounded.
type SLOEvaluation struct {
	ID                   int64      `json:"id"`
	SLOID                int64      `json:"sloId"`
	ProjectID            int64      `json:"projectId"`
	DefinitionVersion    int        `json:"definitionVersion"`
	Status               string     `json:"status"`
	ObservedPercent      *float64   `json:"observedPercent,omitempty"`
	ErrorBudgetRemaining *float64   `json:"errorBudgetRemaining,omitempty"`
	BurnRate             *float64   `json:"burnRate,omitempty"`
	GoodEvents           int64      `json:"goodEvents"`
	TotalEvents          int64      `json:"totalEvents"`
	WindowStartedAt      time.Time  `json:"windowStartedAt"`
	WindowEndedAt        time.Time  `json:"windowEndedAt"`
	ObservedAt           *time.Time `json:"observedAt,omitempty"`
	Provenance           string     `json:"provenance"`
	EvaluatedAt          time.Time  `json:"evaluatedAt"`
}

// SLOMetricAggregate is the only evidence an evaluator needs from the metrics
// backend. It intentionally contains counts and a freshness timestamp, never
// raw telemetry labels, URLs, traces, or individual request data.
type SLOMetricAggregate struct {
	GoodEvents  int64
	TotalEvents int64
	ObservedAt  *time.Time
	Provenance  string
}
