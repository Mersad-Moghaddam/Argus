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

// HeartbeatLivenessEvaluator turns late and missing job heartbeats into one
// source-scoped incident. It intentionally never updates LastOutcome: that
// field records the job's own reported result, not Argus' liveness judgement.
type HeartbeatLivenessEvaluator struct {
	heartbeats ports.HeartbeatStore
	incidents  ports.ProjectIncidentStore
}

func NewHeartbeatLivenessEvaluator(heartbeats ports.HeartbeatStore, incidents ports.ProjectIncidentStore) *HeartbeatLivenessEvaluator {
	return &HeartbeatLivenessEvaluator{heartbeats: heartbeats, incidents: incidents}
}

func (e *HeartbeatLivenessEvaluator) EvaluateAll(ctx context.Context, now time.Time) error {
	for after := int64(0); ; {
		items, err := e.heartbeats.ListHeartbeatMonitorsForEvaluation(ctx, 100, after)
		if err != nil {
			return err
		}
		for _, monitor := range items {
			after = monitor.ID
			if err = e.syncIncident(ctx, monitor, heartbeatLivenessState(monitor, now), now); err != nil {
				return err
			}
		}
		if len(items) < 100 {
			return nil
		}
	}
}

func (e *HeartbeatLivenessEvaluator) syncIncident(ctx context.Context, monitor models.HeartbeatMonitor, state string, now time.Time) error {
	if e.incidents == nil {
		return nil
	}
	key := fmt.Sprintf("heartbeat:%d", monitor.ID)
	open, err := e.incidents.GetOpenProjectIncident(ctx, monitor.ProjectID, "heartbeat", key)
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
	evidence, _ := json.Marshal(map[string]any{"heartbeatId": monitor.ID, "livenessState": state, "expectedIntervalSeconds": monitor.ExpectedIntervalSeconds, "gracePeriodSeconds": monitor.GracePeriodSeconds, "lastReceivedAt": monitor.LastReceivedAt, "evaluatedAt": now.Format(time.RFC3339)})
	_, err = e.incidents.CreateProjectIncident(ctx, models.ProjectIncident{ProjectID: monitor.ProjectID, Source: "heartbeat", SourceKey: key, Title: fmt.Sprintf("Heartbeat %q is %s", monitor.Name, state), Evidence: string(evidence), StartedAt: now})
	return err
}

func heartbeatLivenessState(monitor models.HeartbeatMonitor, now time.Time) string {
	if monitor.RevokedAt != nil {
		return "revoked"
	}
	if monitor.LastReceivedAt == nil {
		return "missing"
	}
	age := now.Sub(*monitor.LastReceivedAt)
	if age <= time.Duration(monitor.ExpectedIntervalSeconds)*time.Second {
		return "healthy"
	}
	if age <= time.Duration(monitor.ExpectedIntervalSeconds+monitor.GracePeriodSeconds)*time.Second {
		return "late"
	}
	return "missing"
}

func (p *Processor) SetHeartbeatLivenessEvaluator(e *HeartbeatLivenessEvaluator) {
	p.heartbeatLiveness = e
}
func (p *Processor) RegisterHeartbeatLivenessTasks(mux *asynq.ServeMux) {
	if p.heartbeatLiveness != nil {
		mux.HandleFunc(TypeEvaluateHeartbeatLiveness, p.HandleEvaluateHeartbeatLiveness)
	}
}
func (p *Processor) HandleEvaluateHeartbeatLiveness(ctx context.Context, _ *asynq.Task) error {
	if p.heartbeatLiveness == nil {
		return nil
	}
	return p.heartbeatLiveness.EvaluateAll(ctx, time.Now().UTC())
}
