package application

import (
	"argus/internal/models"
	"context"
	"errors"
	"fmt"
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
	now := time.Now().UTC()
	created, err := s.privateAgentResults.RecordPrivateAgentResult(ctx, models.PrivateAgentResult{AgentID: agent.ID, ProjectID: agent.ProjectID, EnvironmentID: agent.EnvironmentID, Outcome: outcome, Summary: strings.TrimSpace(summary), ReceivedAt: now}, idempotencyKey)
	if err != nil || !created || s.projectIncidents == nil {
		return created, err
	}
	key := fmt.Sprintf("agent:%d:result", agent.ID)
	open, err := s.projectIncidents.GetOpenProjectIncident(ctx, agent.ProjectID, "private_agent_result", key)
	if err != nil {
		return created, err
	}
	if outcome == "success" {
		if open != nil {
			err = s.projectIncidents.ResolveProjectIncident(ctx, open.ID, now)
		}
		return created, err
	}
	if open == nil {
		_, err = s.projectIncidents.CreateProjectIncident(ctx, models.ProjectIncident{ProjectID: agent.ProjectID, Source: "private_agent_result", SourceKey: key, Title: fmt.Sprintf("Private agent %q reported failure", agent.Name), Evidence: fmt.Sprintf(`{"agentId":%d,"outcome":"failure"}`, agent.ID), StartedAt: now})
	}
	return created, err
}
