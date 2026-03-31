package models

import "time"

const (
	MonitorTypeHTTPStatus = "http_status"
	MonitorTypeKeyword    = "keyword"
	MonitorTypeHeartbeat  = "heartbeat"
	MonitorTypeTLSExpiry  = "tls_expiry"
)

// Website represents a monitored website.
type Website struct {
	ID                     int64      `json:"id"`
	URL                    string     `json:"url"`
	HealthCheckURL         *string    `json:"healthCheckUrl,omitempty"`
	CheckInterval          int        `json:"checkInterval"`
	MonitorType            string     `json:"monitorType"`
	ExpectedKeyword        *string    `json:"expectedKeyword,omitempty"`
	TLSExpiryThresholdDays int        `json:"tlsExpiryThresholdDays"`
	HeartbeatGraceSeconds  int        `json:"heartbeatGraceSeconds"`
	Status                 string     `json:"status"`
	LastCheckedAt          *time.Time `json:"lastCheckedAt,omitempty"`
	NextCheckAt            time.Time  `json:"nextCheckAt"`
	LastStatusCode         int        `json:"lastStatusCode"`
	LastLatencyMS          int        `json:"lastLatencyMs"`
	StatusPageID           *int64     `json:"statusPageId,omitempty"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}
