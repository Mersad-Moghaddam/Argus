package application

import (
	"context"
	"errors"
	"testing"
)

func TestPrivateAgentLifecycleIsEnvironmentBoundAndRevocable(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	p, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "Private"})
	if err != nil {
		t.Fatal(err)
	}
	envs, _ := h.service.ListProjectEnvironments(ctx, p.ID)
	issued, err := h.service.CreatePrivateAgent(ctx, p.ID, 1, CreatePrivateAgentInput{Name: "edge", EnvironmentID: envs[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	if issued.EnrollmentToken == "" || len(issued.Agent.TokenHash) != 32 {
		t.Fatalf("unsafe issue: %+v", issued)
	}
	if issued.Agent.ExpectedIntervalSeconds != 60 || issued.Agent.Status != "offline" {
		t.Fatalf("unexpected liveness defaults: %+v", issued.Agent)
	}
	if items, err := h.service.ListPrivateAgents(ctx, p.ID); err != nil || len(items) != 1 || items[0].TokenHash != nil {
		t.Fatalf("list should return one safe agent: %+v %v", items, err)
	}
	agent, err := h.service.AuthenticatePrivateAgent(ctx, issued.EnrollmentToken, "1.0.0")
	if err != nil || agent.ProjectID != p.ID || agent.EnvironmentID != envs[0].ID || agent.LastSeenAt == nil {
		t.Fatalf("auth: %+v %v", agent, err)
	}
	if items, err := h.service.ListPrivateAgents(ctx, p.ID); err != nil || items[0].Status != "healthy" {
		t.Fatalf("expected healthy agent: %+v %v", items, err)
	}
	if err = h.service.RevokePrivateAgent(ctx, p.ID, agent.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = h.service.AuthenticatePrivateAgent(ctx, issued.EnrollmentToken, "1.0.1"); !errors.Is(err, ErrPrivateAgentNotFound) {
		t.Fatalf("revoked token: %v", err)
	}
}
