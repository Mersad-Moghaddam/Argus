package worker

import (
	"context"
	"testing"
	"time"

	"argus/internal/models"
	"argus/internal/testsupport"
)

func TestHeartbeatLivenessEvaluatorCreatesAndResolvesOneIncident(t *testing.T) {
	stores := testsupport.NewStores()
	now := time.Now().UTC()
	id, err := stores.Heartbeats.CreateHeartbeatMonitor(context.Background(), models.HeartbeatMonitor{ProjectID: 7, Name: "nightly backup", ExpectedIntervalSeconds: 60, GracePeriodSeconds: 60})
	if err != nil {
		t.Fatal(err)
	}
	e := NewHeartbeatLivenessEvaluator(stores.Heartbeats, stores.ProjectIncidents)
	if err = e.EvaluateAll(context.Background(), now); err != nil {
		t.Fatal(err)
	}
	open, err := stores.ProjectIncidents.ListProjectIncidents(context.Background(), 7, "open", 10, 0)
	if err != nil || len(open) != 1 || open[0].Source != "heartbeat" {
		t.Fatalf("open heartbeat incident: %v %#v", err, open)
	}
	if err = e.EvaluateAll(context.Background(), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	open, err = stores.ProjectIncidents.ListProjectIncidents(context.Background(), 7, "open", 10, 0)
	if err != nil || len(open) != 1 {
		t.Fatalf("heartbeat incident duplicated: %v %#v", err, open)
	}
	if err = stores.Heartbeats.TouchHeartbeatMonitor(context.Background(), id, now.Add(2*time.Minute), "success"); err != nil {
		t.Fatal(err)
	}
	if err = e.EvaluateAll(context.Background(), now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	resolved, err := stores.ProjectIncidents.ListProjectIncidents(context.Background(), 7, "resolved", 10, 0)
	if err != nil || len(resolved) != 1 || resolved[0].ResolvedAt == nil {
		t.Fatalf("resolved heartbeat incident: %v %#v", err, resolved)
	}
}
