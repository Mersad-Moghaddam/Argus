package models

import "time"

// TelemetryCredential is an opaque, tenant-bound credential for OTLP ingest.
// TokenHash is deliberately never serialized; TokenPrefix is only an operator
// aid for identifying which credential is being rotated or revoked.
type TelemetryCredential struct {
	ID                 int64      `json:"id"`
	ProjectID          int64      `json:"projectId"`
	EnvironmentID      int64      `json:"environmentId"`
	CreatedByUserID    int64      `json:"createdByUserId"`
	Name               string     `json:"name"`
	TokenPrefix        string     `json:"tokenPrefix"`
	TokenHash          []byte     `json:"-"`
	Scopes             string     `json:"scopes"`
	RateLimitPerMinute int        `json:"rateLimitPerMinute"`
	ExpiresAt          *time.Time `json:"expiresAt,omitempty"`
	RevokedAt          *time.Time `json:"revokedAt,omitempty"`
	LastUsedAt         *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// IssuedTelemetryCredential pairs the display-safe credential record with the
// secret shown once at creation or rotation. The secret is not persisted.
type IssuedTelemetryCredential struct {
	Credential TelemetryCredential `json:"credential"`
	Token      string              `json:"token"`
}
