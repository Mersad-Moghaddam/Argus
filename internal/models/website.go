package models

import "time"

// Website represents a monitored website.
type Website struct {
	ID             int64      `json:"id"`
	URL            string     `json:"url"`
	HealthCheckURL *string    `json:"healthCheckUrl,omitempty"`
	CheckInterval  int        `json:"checkInterval"`
	Status         string     `json:"status"`
	LastCheckedAt  *time.Time `json:"lastCheckedAt,omitempty"`
	NextCheckAt    time.Time  `json:"nextCheckAt"`
	LastStatusCode int        `json:"lastStatusCode"`
	LastLatencyMS  int        `json:"lastLatencyMs"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}
