package models

import "time"

// Website represents a monitored website.
type Website struct {
	ID             int64      `json:"id"`
	URL            string     `json:"url"`
	CheckInterval  int        `json:"checkInterval"`
	Status         string     `json:"status"`
	LastCheckedAt  *time.Time `json:"lastCheckedAt,omitempty"`
	NextCheckAt    time.Time  `json:"nextCheckAt"`
	LastStatusCode int        `json:"lastStatusCode"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}
