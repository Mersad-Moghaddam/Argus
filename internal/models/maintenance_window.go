package models

import "time"

// MaintenanceWindow suppresses alerts for a period.
type MaintenanceWindow struct {
	ID         int64     `json:"id"`
	WebsiteID  *int64    `json:"websiteId,omitempty"`
	StartsAt   time.Time `json:"startsAt"`
	EndsAt     time.Time `json:"endsAt"`
	MuteAlerts bool      `json:"muteAlerts"`
	Reason     *string   `json:"reason,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
