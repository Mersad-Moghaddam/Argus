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
	agent, err := h.service.AuthenticatePrivateAgent(ctx, issued.EnrollmentToken, "1.0.0")
	if err != nil || agent.ProjectID != p.ID || agent.EnvironmentID != envs[0].ID || agent.LastSeenAt == nil {
		t.Fatalf("auth: %+v %v", agent, err)
	}
	if err = h.service.privateAgents.RevokePrivateAgent(ctx, agent.ID, *agent.LastSeenAt); err != nil {
		t.Fatal(err)
	}
	if _, err = h.service.AuthenticatePrivateAgent(ctx, issued.EnrollmentToken, "1.0.1"); !errors.Is(err, ErrPrivateAgentNotFound) {
		t.Fatalf("revoked token: %v", err)
	}
}
