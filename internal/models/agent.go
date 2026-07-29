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
	LivenessState           string     `json:"-"`
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

// PrivateAgentResult is deliberately bounded operational evidence. It never
// carries URLs, request headers, payloads, stack traces, or arbitrary logs.
type PrivateAgentResult struct {
	ID            int64     `json:"id"`
	AgentID       int64     `json:"agentId"`
	ProjectID     int64     `json:"projectId"`
	EnvironmentID int64     `json:"environmentId"`
	AssignmentID  *int64    `json:"assignmentId,omitempty"`
	Outcome       string    `json:"outcome"`
	Summary       string    `json:"summary,omitempty"`
	ReceivedAt    time.Time `json:"receivedAt"`
}

// PrivateAgentAssignment is a deliberately narrow, environment-bound unit of
// work. It is not a general remote-command channel: only a pre-approved GET
// or HEAD target can be assigned, and the agent receives it through a signed,
// expiry-bound configuration document.
type PrivateAgentAssignment struct {
	ID            int64      `json:"id"`
	ProjectID     int64      `json:"projectId"`
	EnvironmentID int64      `json:"environmentId"`
	RouteID       int64      `json:"routeId"`
	Name          string     `json:"name"`
	Method        string     `json:"method"`
	Target        string     `json:"target"`
	IntervalSecs  int        `json:"intervalSeconds"`
	TimeoutMS     int        `json:"timeoutMs"`
	Enabled       bool       `json:"enabled"`
	CreatedByID   int64      `json:"createdByUserId"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	RevokedAt     *time.Time `json:"revokedAt,omitempty"`
}
