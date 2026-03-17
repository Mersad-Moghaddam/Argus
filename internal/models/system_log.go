package models

import "time"

// SystemLog is a user-visible operational event.
type SystemLog struct {
	Timestamp time.Time         `json:"timestamp"`
	Level     string            `json:"level"`
	Source    string            `json:"source"`
	Action    string            `json:"action"`
	Message   string            `json:"message"`
	WebsiteID *int64            `json:"websiteId,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}
