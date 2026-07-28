package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
)

const (
	defaultTelemetryCredentialLifetime = 90 * 24 * time.Hour
	maxTelemetryCredentialLifetime     = 365 * 24 * time.Hour
	defaultTelemetryRateLimit          = 600
)

var ErrTelemetryCredentialNotFound = errors.New("telemetry credential not found")

type CreateTelemetryCredentialInput struct {
	Name               string
	EnvironmentID      int64
	ExpiresIn          time.Duration
	RateLimitPerMinute int
}

// CreateTelemetryCredential issues an opaque secret once. It derives all
// tenant attribution from the authorized project and the selected project
// environment, never from caller-controlled OTLP attributes.
func (s *Service) CreateTelemetryCredential(ctx context.Context, projectID, userID int64, input CreateTelemetryCredentialInput) (models.IssuedTelemetryCredential, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 120 || input.EnvironmentID <= 0 {
		return models.IssuedTelemetryCredential{}, domain.ErrInvalidInput
	}
	if !s.projectHasEnvironment(ctx, projectID, input.EnvironmentID) {
		return models.IssuedTelemetryCredential{}, domain.ErrInvalidInput
	}
	lifetime := input.ExpiresIn
	if lifetime <= 0 {
		lifetime = defaultTelemetryCredentialLifetime
	}
	if lifetime > maxTelemetryCredentialLifetime {
		return models.IssuedTelemetryCredential{}, domain.ErrInvalidInput
	}
	rateLimit := input.RateLimitPerMinute
	if rateLimit <= 0 {
		rateLimit = defaultTelemetryRateLimit
	}
	if rateLimit > 10_000 {
		return models.IssuedTelemetryCredential{}, domain.ErrInvalidInput
	}

	raw, err := newTelemetryCredentialToken()
	if err != nil {
		return models.IssuedTelemetryCredential{}, err
	}
	hash := sha256.Sum256([]byte(raw))
	expiresAt := time.Now().UTC().Add(lifetime)
	credential := models.TelemetryCredential{
		ProjectID: projectID, EnvironmentID: input.EnvironmentID, CreatedByUserID: userID,
		Name: name, TokenPrefix: raw[:18], TokenHash: hash[:],
		Scopes: "metrics:write,traces:write", RateLimitPerMinute: rateLimit, ExpiresAt: &expiresAt,
	}
	id, err := s.telemetryCredentials.CreateTelemetryCredential(ctx, credential)
	if err != nil {
		return models.IssuedTelemetryCredential{}, err
	}
	credential.ID = id
	return models.IssuedTelemetryCredential{Credential: credential, Token: raw}, nil
}

func (s *Service) ListTelemetryCredentials(ctx context.Context, projectID int64) ([]models.TelemetryCredential, error) {
	return s.telemetryCredentials.ListTelemetryCredentials(ctx, projectID)
}

func (s *Service) RevokeTelemetryCredential(ctx context.Context, projectID, credentialID int64) error {
	credential, err := s.telemetryCredentials.GetTelemetryCredentialByID(ctx, credentialID)
	if err != nil {
		return err
	}
	if credential == nil || credential.ProjectID != projectID {
		return ErrTelemetryCredentialNotFound
	}
	if credential.RevokedAt != nil {
		return nil
	}
	return s.telemetryCredentials.RevokeTelemetryCredential(ctx, credentialID, time.Now().UTC())
}

func (s *Service) RotateTelemetryCredential(ctx context.Context, projectID, userID, credentialID int64, input CreateTelemetryCredentialInput) (models.IssuedTelemetryCredential, error) {
	credential, err := s.telemetryCredentials.GetTelemetryCredentialByID(ctx, credentialID)
	if err != nil {
		return models.IssuedTelemetryCredential{}, err
	}
	if credential == nil || credential.ProjectID != projectID {
		return models.IssuedTelemetryCredential{}, ErrTelemetryCredentialNotFound
	}
	if input.EnvironmentID == 0 {
		input.EnvironmentID = credential.EnvironmentID
	}
	if strings.TrimSpace(input.Name) == "" {
		input.Name = credential.Name
	}
	issued, err := s.CreateTelemetryCredential(ctx, projectID, userID, input)
	if err != nil {
		return models.IssuedTelemetryCredential{}, err
	}
	if err = s.RevokeTelemetryCredential(ctx, projectID, credentialID); err != nil {
		return models.IssuedTelemetryCredential{}, err
	}
	return issued, nil
}

// AuthenticateTelemetryCredential will be used by the OTLP boundary. It
// deliberately returns only server-bound attribution and enforces expiry and
// revocation before any signal is accepted.
func (s *Service) AuthenticateTelemetryCredential(ctx context.Context, raw string) (*models.TelemetryCredential, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrTelemetryCredentialNotFound
	}
	hash := sha256.Sum256([]byte(raw))
	credential, err := s.telemetryCredentials.GetTelemetryCredentialByHash(ctx, hash[:])
	if err != nil {
		return nil, err
	}
	if credential == nil || credential.RevokedAt != nil || credential.ExpiresAt == nil || !credential.ExpiresAt.After(time.Now().UTC()) {
		return nil, ErrTelemetryCredentialNotFound
	}
	usedAt := time.Now().UTC()
	if err = s.telemetryCredentials.TouchTelemetryCredential(ctx, credential.ID, usedAt); err != nil {
		return nil, err
	}
	credential.LastUsedAt = &usedAt
	return credential, nil
}

func (s *Service) projectHasEnvironment(ctx context.Context, projectID, environmentID int64) bool {
	environments, err := s.projects.ListProjectEnvironments(ctx, projectID)
	if err != nil {
		return false
	}
	for _, environment := range environments {
		if environment.ID == environmentID {
			return true
		}
	}
	return false
}

func newTelemetryCredentialToken() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return "argus_otlp_" + base64.RawURLEncoding.EncodeToString(secret), nil
}
