package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
)

func TestHeartbeatLifecycleIsProjectBoundRotatableAndIdempotent(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "Scheduled work"})
	if err != nil {
		t.Fatal(err)
	}
	environments, err := h.service.ListProjectEnvironments(ctx, project.ID)
	if err != nil || len(environments) != 1 {
		t.Fatalf("environment: %v %+v", err, environments)
	}
	issued, err := h.service.CreateHeartbeatMonitor(ctx, project.ID, 1, CreateHeartbeatMonitorInput{Name: "nightly backup", EnvironmentID: environments[0].ID, ExpectedIntervalSeconds: 300, GracePeriodSeconds: 120})
	if err != nil {
		t.Fatal(err)
	}
	if issued.Token == "" || len(issued.Monitor.TokenHash) != 32 || issued.Monitor.TokenPrefix == "" {
		t.Fatalf("unsafe heartbeat issue result: %+v", issued)
	}
	monitor, accepted, err := h.service.ReceiveHeartbeat(ctx, issued.Token, "run-2026-07-29-0001", "success")
	if err != nil || !accepted || monitor.LastReceivedAt == nil {
		t.Fatalf("receive: monitor=%+v accepted=%v err=%v", monitor, accepted, err)
	}
	_, accepted, err = h.service.ReceiveHeartbeat(ctx, issued.Token, "run-2026-07-29-0001", "success")
	if err != nil || accepted {
		t.Fatalf("replay must be accepted without refreshing state: accepted=%v err=%v", accepted, err)
	}
	if err = h.service.RevokeHeartbeatMonitor(ctx, project.ID, issued.Monitor.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err = h.service.ReceiveHeartbeat(ctx, issued.Token, "run-2026-07-29-0002", "success"); !errors.Is(err, ErrHeartbeatMonitorNotFound) {
		t.Fatalf("revoked token must fail, got %v", err)
	}
}

func TestHeartbeatRejectsForeignEnvironmentAndReportsLiveness(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	projectA, _ := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "A"})
	projectB, _ := h.service.CreateProject(ctx, 2, CreateProjectInput{Name: "B"})
	foreign, _ := h.service.CreateProjectEnvironment(ctx, projectB.ID, CreateEnvironmentInput{Name: "staging"})
	if _, err := h.service.CreateHeartbeatMonitor(ctx, projectA.ID, 1, CreateHeartbeatMonitorInput{Name: "bad", EnvironmentID: foreign.ID}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("foreign environment must fail, got %v", err)
	}
	if heartbeatState(models.HeartbeatMonitor{}, time.Now()) != "missing" {
		t.Fatal("no signal must be missing")
	}
	seen := time.Now().Add(-6 * time.Minute)
	monitor := models.HeartbeatMonitor{ExpectedIntervalSeconds: 300, GracePeriodSeconds: 120, LastReceivedAt: &seen}
	if heartbeatState(monitor, time.Now()) != "late" {
		t.Fatal("expected late state")
	}
	seen = time.Now().Add(-8 * time.Minute)
	monitor.LastReceivedAt = &seen
	if heartbeatState(monitor, time.Now()) != "missing" {
		t.Fatal("expected missing state")
	}
}
