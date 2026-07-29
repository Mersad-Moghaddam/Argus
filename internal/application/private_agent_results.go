package application

import (
	"argus/internal/models"
	"context"
	"errors"
	"strings"
	"time"
)

func (s *Service) RecordPrivateAgentResult(ctx context.Context, token, version, idempotencyKey, outcome, summary string) (bool, error) {
	if s.privateAgentResults == nil {
		return false, errors.New("agent result service unavailable")
	}
	if len(strings.TrimSpace(idempotencyKey)) < 16 || len(idempotencyKey) > 200 || (outcome != "success" && outcome != "failure") || len(summary) > 240 {
		return false, ErrPrivateAgentNotFound
	}
	agent, err := s.AuthenticatePrivateAgent(ctx, token, version)
	if err != nil {
		return false, err
	}
	return s.privateAgentResults.RecordPrivateAgentResult(ctx, models.PrivateAgentResult{AgentID: agent.ID, ProjectID: agent.ProjectID, EnvironmentID: agent.EnvironmentID, Outcome: outcome, Summary: strings.TrimSpace(summary), ReceivedAt: time.Now().UTC()}, idempotencyKey)
}
