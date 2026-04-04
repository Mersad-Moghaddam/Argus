package models

import "time"

// AlertChannel defines where alerts are delivered.
type AlertChannel struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	ChannelType string    `json:"channelType"`
	Target      string    `json:"target"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
