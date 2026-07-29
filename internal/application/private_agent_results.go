package application

import (
	"argus/internal/models"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrInvalidPrivateAgentResult = errors.New("invalid private agent result")

func (s *Service) RecordPrivateAgentResult(ctx context.Context, token, version, idempotencyKey string, assignmentID int64, outcome, summary string) (bool, error) {
	if s.privateAgentResults == nil {
		return false, errors.New("agent result service unavailable")
	}
	if len(strings.TrimSpace(idempotencyKey)) < 16 || len(idempotencyKey) > 200 || (outcome != "success" && outcome != "failure") || len(summary) > 240 {
		return false, ErrInvalidPrivateAgentResult
	}
	agent, err := s.AuthenticatePrivateAgent(ctx, token, version)
	if err != nil {
		return false, err
	}
	var assignment *models.PrivateAgentAssignment
	if assignmentID > 0 {
		if s.privateAgentAssignments == nil {
			return false, ErrInvalidPrivateAgentResult
		}
		items, listErr := s.privateAgentAssignments.ListPrivateAgentAssignmentsForEnvironment(ctx, agent.ProjectID, agent.EnvironmentID)
		if listErr != nil {
			return false, listErr
		}
		for i := range items {
			if items[i].ID == assignmentID && items[i].Enabled && items[i].RevokedAt == nil {
				assignment = &items[i]
				break
			}
		}
		if assignment == nil {
			return false, ErrInvalidPrivateAgentResult
		}
	}
	now := time.Now().UTC()
	result := models.PrivateAgentResult{AgentID: agent.ID, ProjectID: agent.ProjectID, EnvironmentID: agent.EnvironmentID, Outcome: outcome, Summary: strings.TrimSpace(summary), ReceivedAt: now}
	if assignment != nil {
		result.AssignmentID = &assignment.ID
	}
	created, err := s.privateAgentResults.RecordPrivateAgentResult(ctx, result, idempotencyKey)
	if err != nil || !created || s.projectIncidents == nil {
		return created, err
	}
	key := fmt.Sprintf("agent:%d:result", agent.ID)
	if assignment != nil {
		key = fmt.Sprintf("agent:%d:assignment:%d", agent.ID, assignment.ID)
	}
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
		title := fmt.Sprintf("Private agent %q reported failure", agent.Name)
		evidence := fmt.Sprintf(`{"agentId":%d,"outcome":"failure"}`, agent.ID)
		if assignment != nil {
			title = fmt.Sprintf("Private agent assignment %q reported failure", assignment.Name)
			evidence = fmt.Sprintf(`{"agentId":%d,"assignmentId":%d,"outcome":"failure"}`, agent.ID, assignment.ID)
		}
		_, err = s.projectIncidents.CreateProjectIncident(ctx, models.ProjectIncident{ProjectID: agent.ProjectID, Source: "private_agent_result", SourceKey: key, Title: title, Evidence: evidence, StartedAt: now})
	}
	return created, err
}
