package models

import "time"

// User is an authenticated account that owns and collaborates on projects.
type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// AuthToken is an opaque bearer token issued at login/register.
type AuthToken struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"userId"`
	TokenHash  string     `json:"-"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	Current    bool       `json:"current,omitempty"`
}

// PasswordRecoveryToken is an opaque, single-use reset credential. Only its
// SHA-256 hash is persisted; raw values are handed directly to a configured
// delivery integration and are never returned from the control-plane API.
type PasswordRecoveryToken struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"userId"`
	TokenHash string     `json:"-"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

const (
	ProjectRoleOwner  = "owner"
	ProjectRoleEditor = "editor"
	ProjectRoleViewer = "viewer"
)

// ProjectMember links a user to a project with a role.
type ProjectMember struct {
	ID        int64     `json:"id"`
	ProjectID int64     `json:"projectId"`
	UserID    int64     `json:"userId"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}
