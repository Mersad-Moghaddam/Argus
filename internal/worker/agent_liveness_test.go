package worker

import (
	"argus/internal/models"
	"argus/internal/testsupport"
	"context"
	"testing"
	"time"
)

func TestAgentLivenessEvaluatorEmitsOnlyTransitions(t *testing.T) {
	stores := testsupport.NewStores()
	now := time.Now().UTC()
	seen := now.Add(-3 * time.Minute)
	id, err := stores.PrivateAgents.CreatePrivateAgent(context.Background(), models.PrivateAgent{ProjectID: 7, ExpectedIntervalSeconds: 60, LastSeenAt: &seen, LivenessState: "healthy"})
	if err != nil {
		t.Fatal(err)
	}
	e := NewAgentLivenessEvaluator(stores.PrivateAgents, stores.ProjectIncidents, stores.Outbox)
	if err = e.EvaluateAll(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if got := stores.Outbox.EventTypes(); len(got) != 1 || got[0] != "agent_liveness_changed" {
		t.Fatalf("events: %#v", got)
	}
	if err = e.EvaluateAll(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if got := stores.Outbox.EventTypes(); len(got) != 1 {
		t.Fatalf("repeated state emitted: %#v", got)
	}
	if id == 0 {
		t.Fatal("missing agent")
	}
	incidents, err := stores.ProjectIncidents.ListProjectIncidents(context.Background(), 7, "open", 10, 0)
	if err != nil || len(incidents) != 1 || incidents[0].Source != "private_agent" {
		t.Fatalf("open agent incident: %v %#v", err, incidents)
	}
}

func TestAgentLivenessEvaluatorResolvesIncidentWhenAgentRecovers(t *testing.T) {
	stores := testsupport.NewStores()
	now := time.Now().UTC()
	seen := now.Add(-3 * time.Minute)
	id, err := stores.PrivateAgents.CreatePrivateAgent(context.Background(), models.PrivateAgent{ProjectID: 7, Name: "private edge", ExpectedIntervalSeconds: 60, LastSeenAt: &seen, LivenessState: "healthy"})
	if err != nil {
		t.Fatal(err)
	}
	e := NewAgentLivenessEvaluator(stores.PrivateAgents, stores.ProjectIncidents, stores.Outbox)
	if err = e.EvaluateAll(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	if err = stores.PrivateAgents.TouchPrivateAgent(context.Background(), id, "v1", now); err != nil {
		t.Fatal(err)
	}
	if err = e.EvaluateAll(context.Background(), now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	resolved, err := stores.ProjectIncidents.ListProjectIncidents(context.Background(), 7, "resolved", 10, 0)
	if err != nil || len(resolved) != 1 || resolved[0].ResolvedAt == nil {
		t.Fatalf("resolved agent incident: %v %#v", err, resolved)
	}
}

func TestAgentLivenessEvaluatorOpensIncidentForInitialOfflineAgent(t *testing.T) {
	stores := testsupport.NewStores()
	_, err := stores.PrivateAgents.CreatePrivateAgent(context.Background(), models.PrivateAgent{ProjectID: 7, Name: "new edge", ExpectedIntervalSeconds: 60, LivenessState: "offline"})
	if err != nil {
		t.Fatal(err)
	}
	e := NewAgentLivenessEvaluator(stores.PrivateAgents, stores.ProjectIncidents, stores.Outbox)
	if err = e.EvaluateAll(context.Background(), time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	open, err := stores.ProjectIncidents.ListProjectIncidents(context.Background(), 7, "open", 10, 0)
	if err != nil || len(open) != 1 {
		t.Fatalf("initial offline incident: %v %#v", err, open)
	}
}
