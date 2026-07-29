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

func TestPrivateAgentResultsDriveAReplaySafeIncidentLifecycle(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "Agent results"})
	if err != nil {
		t.Fatal(err)
	}
	environments, err := h.service.ListProjectEnvironments(ctx, project.ID)
	if err != nil || len(environments) != 1 {
		t.Fatalf("environments: %#v %v", environments, err)
	}
	issued, err := h.service.CreatePrivateAgent(ctx, project.ID, 1, CreatePrivateAgentInput{Name: "edge", EnvironmentID: environments[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	created, err := h.service.RecordPrivateAgentResult(ctx, issued.EnrollmentToken, "1.2.3", "agent-result-failure-0001", "failure", "bounded failure")
	if err != nil || !created {
		t.Fatalf("record failure: created=%t err=%v", created, err)
	}
	open, err := h.projectIncidents.ListProjectIncidents(ctx, project.ID, "open", 10, 0)
	if err != nil || len(open) != 1 || open[0].Source != "private_agent_result" {
		t.Fatalf("open result incident: %#v %v", open, err)
	}
	created, err = h.service.RecordPrivateAgentResult(ctx, issued.EnrollmentToken, "1.2.3", "agent-result-failure-0001", "failure", "bounded failure")
	if err != nil || created {
		t.Fatalf("replay failure: created=%t err=%v", created, err)
	}
	created, err = h.service.RecordPrivateAgentResult(ctx, issued.EnrollmentToken, "1.2.3", "agent-result-success-0001", "success", "recovered")
	if err != nil || !created {
		t.Fatalf("record recovery: created=%t err=%v", created, err)
	}
	resolved, err := h.projectIncidents.ListProjectIncidents(ctx, project.ID, "resolved", 10, 0)
	if err != nil || len(resolved) != 1 || resolved[0].ResolvedAt == nil {
		t.Fatalf("resolved result incident: %#v %v", resolved, err)
	}
	if _, err = h.service.RecordPrivateAgentResult(ctx, issued.EnrollmentToken, "1.2.3", "too-short", "success", ""); !errors.Is(err, ErrInvalidPrivateAgentResult) {
		t.Fatalf("invalid result error = %v", err)
	}
}
