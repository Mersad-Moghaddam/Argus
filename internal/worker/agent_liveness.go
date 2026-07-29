package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"argus/internal/domain/ports"
	"argus/internal/models"
	"github.com/hibiken/asynq"
)

// AgentLivenessEvaluator persists state transitions so repeated scheduler
// passes do not generate notification noise.
type AgentLivenessEvaluator struct {
	agents    ports.PrivateAgentStore
	incidents ports.ProjectIncidentStore
	outbox    ports.OutboxStore
}

func NewAgentLivenessEvaluator(agents ports.PrivateAgentStore, incidents ports.ProjectIncidentStore, outbox ports.OutboxStore) *AgentLivenessEvaluator {
	return &AgentLivenessEvaluator{agents: agents, incidents: incidents, outbox: outbox}
}
func (e *AgentLivenessEvaluator) EvaluateAll(ctx context.Context, now time.Time) error {
	for after := int64(0); ; {
		items, err := e.agents.ListPrivateAgentsForEvaluation(ctx, 100, after)
		if err != nil {
			return err
		}
		for _, a := range items {
			after = a.ID
			next := evaluatedAgentState(a, now)
			changed, err := e.agents.SetPrivateAgentLivenessState(ctx, a.ID, next)
			if err != nil {
				return err
			}
			if err = e.syncIncident(ctx, a, next, now); err != nil {
				return err
			}
			if !changed {
				continue
			}
			if e.outbox != nil {
				payload, _ := json.Marshal(map[string]any{"event": "agent_liveness_changed", "projectId": a.ProjectID, "agentId": a.ID, "status": next, "previousStatus": a.LivenessState, "evaluatedAt": now.Format(time.RFC3339)})
				if err = e.outbox.AddEvent(ctx, "agent_liveness_changed", a.ID, fmt.Sprintf("agent:%d:liveness:%s:%s", a.ID, next, now.UTC().Truncate(time.Minute).Format(time.RFC3339)), payload, now); err != nil {
					return err
				}
			}
		}
		if len(items) < 100 {
			return nil
		}
	}
}

// syncIncident records one source-scoped incident for a private agent that is
// no longer healthy. Revocation resolves any active incident because the
// operator deliberately removed that liveness obligation.
func (e *AgentLivenessEvaluator) syncIncident(ctx context.Context, agent models.PrivateAgent, state string, now time.Time) error {
	if e.incidents == nil {
		return nil
	}
	key := fmt.Sprintf("agent:%d", agent.ID)
	open, err := e.incidents.GetOpenProjectIncident(ctx, agent.ProjectID, "private_agent", key)
	if err != nil {
		return err
	}
	if state == "healthy" || state == "revoked" {
		if open != nil {
			return e.incidents.ResolveProjectIncident(ctx, open.ID, now)
		}
		return nil
	}
	if open != nil {
		return nil
	}
	evidence, _ := json.Marshal(map[string]any{
		"agentId": agent.ID, "livenessState": state, "expectedIntervalSeconds": agent.ExpectedIntervalSeconds,
		"lastSeenAt": agent.LastSeenAt, "evaluatedAt": now.Format(time.RFC3339),
	})
	_, err = e.incidents.CreateProjectIncident(ctx, models.ProjectIncident{
		ProjectID: agent.ProjectID,
		Source:    "private_agent", SourceKey: key,
		Title:    fmt.Sprintf("Private agent %q is %s", agent.Name, state),
		Evidence: string(evidence), StartedAt: now,
	})
	return err
}
func evaluatedAgentState(a models.PrivateAgent, now time.Time) string {
	if a.RevokedAt != nil {
		return "revoked"
	}
	if a.LastSeenAt == nil {
		return "offline"
	}
	age := now.Sub(*a.LastSeenAt)
	interval := time.Duration(a.ExpectedIntervalSeconds) * time.Second
	if age <= interval {
		return "healthy"
	}
	if age <= interval*2 {
		return "stale"
	}
	return "offline"
}
func (p *Processor) SetAgentLivenessEvaluator(e *AgentLivenessEvaluator) { p.agentLiveness = e }
func (p *Processor) RegisterAgentLivenessTasks(mux *asynq.ServeMux) {
	if p.agentLiveness != nil {
		mux.HandleFunc(TypeEvaluateAgentLiveness, p.HandleEvaluateAgentLiveness)
	}
}
func (p *Processor) HandleEvaluateAgentLiveness(ctx context.Context, _ *asynq.Task) error {
	if p.agentLiveness == nil {
		return nil
	}
	return p.agentLiveness.EvaluateAll(ctx, time.Now().UTC())
}
