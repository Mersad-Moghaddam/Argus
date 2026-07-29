package models

import "time"

// ProjectIncident is source-agnostic operational lifecycle state. Route
// incidents remain a compatibility record; this model is for agents, jobs,
// SLOs, pipelines, and future non-route producers.
type ProjectIncident struct {
	ID               int64      `json:"id"`
	ProjectID        int64      `json:"projectId"`
	Source           string     `json:"source"`
	SourceKey        string     `json:"sourceKey"`
	State            string     `json:"state"`
	Title            string     `json:"title"`
	Evidence         string     `json:"evidence,omitempty"`
	StartedAt        time.Time  `json:"startedAt"`
	AcknowledgedAt   *time.Time `json:"acknowledgedAt,omitempty"`
	AcknowledgedByID *int64     `json:"acknowledgedByUserId,omitempty"`
	ResolvedAt       *time.Time `json:"resolvedAt,omitempty"`
}
