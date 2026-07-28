package models

import "time"

// ProjectEnvironment is a tenant-scoped deployment context for route and
// telemetry attribution. Canonical URLs are persisted server-side only.
type ProjectEnvironment struct {
	ID               int64     `json:"id"`
	ProjectID        int64     `json:"projectId"`
	Name             string    `json:"name"`
	CanonicalBaseURL string    `json:"canonicalBaseUrl,omitempty"`
	CanonicalOrigin  string    `json:"canonicalOrigin,omitempty"`
	IsDefault        bool      `json:"isDefault"`
	CreatedAt        time.Time `json:"createdAt"`
	UpdatedAt        time.Time `json:"updatedAt"`
}
