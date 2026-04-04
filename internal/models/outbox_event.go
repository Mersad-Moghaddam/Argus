package models

import "time"

type OutboxEvent struct {
	ID              int64      `json:"id"`
	EventType       string     `json:"eventType"`
	AggregateID     int64      `json:"aggregateId"`
	DedupeKey       string     `json:"dedupeKey"`
	Payload         []byte     `json:"payload"`
	Status          string     `json:"status"`
	AvailableAt     time.Time  `json:"availableAt"`
	ProcessedAt     *time.Time `json:"processedAt,omitempty"`
	LastError       *string    `json:"lastError,omitempty"`
	RetryCount      int        `json:"retryCount"`
	CreatedAt       time.Time  `json:"createdAt"`
	LastAttemptedAt *time.Time `json:"lastAttemptedAt,omitempty"`
}
