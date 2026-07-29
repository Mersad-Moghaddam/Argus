package application

import (
	"argus/internal/domain"
	"argus/internal/models"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"
)

var ErrPrivateAgentNotFound = errors.New("private agent not found")

type CreatePrivateAgentInput struct {
	Name                    string
	EnvironmentID           int64
	ExpectedIntervalSeconds int
}

type CreatePrivateAgentAssignmentInput struct {
	EnvironmentID int64
	RouteID       int64
	Name          string
	Method        string
	Target        string
	IntervalSecs  int
	TimeoutMS     int
}

func (s *Service) CreatePrivateAgentAssignment(ctx context.Context, projectID, userID int64, in CreatePrivateAgentAssignmentInput) (models.PrivateAgentAssignment, error) {
	if s.privateAgentAssignments == nil || in.EnvironmentID <= 0 || in.RouteID <= 0 || !s.projectHasEnvironment(ctx, projectID, in.EnvironmentID) {
		return models.PrivateAgentAssignment{}, domain.ErrInvalidInput
	}
	route, err := s.routes.GetRouteByID(ctx, in.RouteID)
	if err != nil || route == nil || route.ProjectID != projectID {
		return models.PrivateAgentAssignment{}, domain.ErrInvalidInput
	}
	method := strings.ToUpper(strings.TrimSpace(in.Method))
	if method != "GET" && method != "HEAD" {
		return models.PrivateAgentAssignment{}, domain.ErrInvalidInput
	}
	target := strings.TrimSpace(in.Target)
	u, parseErr := url.Parse(target)
	if parseErr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return models.PrivateAgentAssignment{}, domain.ErrInvalidInput
	}
	if in.IntervalSecs < 15 || in.IntervalSecs > 86400 || in.TimeoutMS < 200 || in.TimeoutMS > 60000 {
		return models.PrivateAgentAssignment{}, domain.ErrInvalidInput
	}
	a := models.PrivateAgentAssignment{ProjectID: projectID, EnvironmentID: in.EnvironmentID, RouteID: in.RouteID, Name: strings.TrimSpace(in.Name), Method: method, Target: u.String(), IntervalSecs: in.IntervalSecs, TimeoutMS: in.TimeoutMS, Enabled: true, CreatedByID: userID}
	if a.Name == "" || len(a.Name) > 120 {
		return models.PrivateAgentAssignment{}, domain.ErrInvalidInput
	}
	id, err := s.privateAgentAssignments.CreatePrivateAgentAssignment(ctx, a)
	if err != nil {
		return a, err
	}
	a.ID = id
	return a, nil
}

func (s *Service) ListPrivateAgentAssignments(ctx context.Context, projectID int64) ([]models.PrivateAgentAssignment, error) {
	if s.privateAgentAssignments == nil {
		return nil, ErrPrivateAgentNotFound
	}
	return s.privateAgentAssignments.ListPrivateAgentAssignments(ctx, projectID)
}

func (s *Service) RevokePrivateAgentAssignment(ctx context.Context, projectID, id int64) error {
	if s.privateAgentAssignments == nil {
		return ErrPrivateAgentNotFound
	}
	return s.privateAgentAssignments.RevokePrivateAgentAssignment(ctx, projectID, id, time.Now().UTC())
}

const (
	defaultPrivateAgentInterval = time.Minute
	minPrivateAgentInterval     = 15 * time.Second
	maxPrivateAgentInterval     = 24 * time.Hour
)

func (s *Service) CreatePrivateAgent(ctx context.Context, projectID, userID int64, in CreatePrivateAgentInput) (models.IssuedPrivateAgent, error) {
	if s.privateAgents == nil {
		return models.IssuedPrivateAgent{}, ErrPrivateAgentNotFound
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 120 || in.EnvironmentID <= 0 || !s.projectHasEnvironment(ctx, projectID, in.EnvironmentID) {
		return models.IssuedPrivateAgent{}, domain.ErrInvalidInput
	}
	interval := time.Duration(in.ExpectedIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = defaultPrivateAgentInterval
	}
	if interval < minPrivateAgentInterval || interval > maxPrivateAgentInterval {
		return models.IssuedPrivateAgent{}, domain.ErrInvalidInput
	}
	raw, err := newPrivateAgentToken()
	if err != nil {
		return models.IssuedPrivateAgent{}, err
	}
	hash := sha256.Sum256([]byte(raw))
	a := models.PrivateAgent{ProjectID: projectID, EnvironmentID: in.EnvironmentID, CreatedByUserID: userID, Name: name, TokenPrefix: raw[:18], TokenHash: hash[:], ExpectedIntervalSeconds: int(interval.Seconds()), Status: "offline"}
	id, err := s.privateAgents.CreatePrivateAgent(ctx, a)
	if err != nil {
		return models.IssuedPrivateAgent{}, err
	}
	a.ID = id
	return models.IssuedPrivateAgent{Agent: a, EnrollmentToken: raw}, nil
}

func (s *Service) ListPrivateAgents(ctx context.Context, projectID int64) ([]models.PrivateAgent, error) {
	if s.privateAgents == nil {
		return nil, ErrPrivateAgentNotFound
	}
	items, err := s.privateAgents.ListPrivateAgents(ctx, projectID)
	for i := range items {
		// A hash is not an API credential, but withholding it keeps agent
		// management responses strictly metadata-only and avoids future misuse.
		items[i].TokenHash = nil
		items[i].Status = privateAgentState(items[i], time.Now().UTC())
	}
	return items, err
}

func privateAgentState(agent models.PrivateAgent, now time.Time) string {
	if agent.RevokedAt != nil {
		return "revoked"
	}
	if agent.LastSeenAt == nil {
		return "offline"
	}
	age := now.Sub(*agent.LastSeenAt)
	interval := time.Duration(agent.ExpectedIntervalSeconds) * time.Second
	if age <= interval {
		return "healthy"
	}
	if age <= interval*2 {
		return "stale"
	}
	return "offline"
}

func (s *Service) RevokePrivateAgent(ctx context.Context, projectID, agentID int64) error {
	if s.privateAgents == nil {
		return ErrPrivateAgentNotFound
	}
	agents, err := s.privateAgents.ListPrivateAgents(ctx, projectID)
	if err != nil {
		return err
	}
	for _, agent := range agents {
		if agent.ID == agentID {
			if agent.RevokedAt != nil {
				return nil
			}
			return s.privateAgents.RevokePrivateAgent(ctx, agentID, time.Now().UTC())
		}
	}
	return ErrPrivateAgentNotFound
}

func (s *Service) AuthenticatePrivateAgent(ctx context.Context, token, version string) (*models.PrivateAgent, error) {
	if s.privateAgents == nil {
		return nil, ErrPrivateAgentNotFound
	}
	hash := sha256.Sum256([]byte(strings.TrimSpace(token)))
	a, err := s.privateAgents.GetPrivateAgentByHash(ctx, hash[:])
	if err != nil {
		return nil, err
	}
	if a == nil || a.RevokedAt != nil {
		return nil, ErrPrivateAgentNotFound
	}
	now := time.Now().UTC()
	if err = s.privateAgents.TouchPrivateAgent(ctx, a.ID, strings.TrimSpace(version), now); err != nil {
		return nil, err
	}
	a.LastSeenAt = &now
	a.Version = strings.TrimSpace(version)
	return a, nil
}
func newPrivateAgentToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "argus_agent_" + base64.RawURLEncoding.EncodeToString(b), nil
}
