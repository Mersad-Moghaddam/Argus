package models

import "time"

// PrivateAgent is an environment-local executor. Its credential can report
// outbound results to Argus but never authorizes the central service to dial
// into the agent's network.
type PrivateAgent struct {
	ID                      int64      `json:"id"`
	ProjectID               int64      `json:"projectId"`
	EnvironmentID           int64      `json:"environmentId"`
	CreatedByUserID         int64      `json:"createdByUserId"`
	Name                    string     `json:"name"`
	TokenPrefix             string     `json:"tokenPrefix"`
	TokenHash               []byte     `json:"-"`
	Version                 string     `json:"version,omitempty"`
	ExpectedIntervalSeconds int        `json:"expectedIntervalSeconds"`
	Status                  string     `json:"status,omitempty"`
	LastSeenAt              *time.Time `json:"lastSeenAt,omitempty"`
	RevokedAt               *time.Time `json:"revokedAt,omitempty"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

type IssuedPrivateAgent struct {
	Agent           PrivateAgent `json:"agent"`
	EnrollmentToken string       `json:"enrollmentToken"`
}
