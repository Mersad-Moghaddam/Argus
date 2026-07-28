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
	e := NewAgentLivenessEvaluator(stores.PrivateAgents, stores.Outbox)
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
}
