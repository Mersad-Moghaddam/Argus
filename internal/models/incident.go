package models

import "time"

// Incident represents downtime lifecycle for a website.
type Incident struct {
	ID                int64      `json:"id"`
	WebsiteID         int64      `json:"websiteId"`
	State             string     `json:"state"`
	StartedAt         time.Time  `json:"startedAt"`
	AcknowledgedAt    *time.Time `json:"acknowledgedAt,omitempty"`
	ResolvedAt        *time.Time `json:"resolvedAt,omitempty"`
	LastFailureReason *string    `json:"lastFailureReason,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}
