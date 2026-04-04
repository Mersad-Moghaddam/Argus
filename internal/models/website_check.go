package models

import "time"

type WebsiteCheck struct {
	ID            int64      `json:"id"`
	WebsiteID     int64      `json:"websiteId"`
	Status        string     `json:"status"`
	StatusCode    int        `json:"statusCode"`
	LatencyMS     int        `json:"latencyMs"`
	FailureReason *string    `json:"failureReason,omitempty"`
	CheckedAt     time.Time  `json:"checkedAt"`
	CreatedAt     time.Time  `json:"createdAt"`
}
