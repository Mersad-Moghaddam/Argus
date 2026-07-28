package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
)

func TestTelemetryCredentialLifecycleBindsTenantAndEnvironment(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project, err := h.service.CreateProject(ctx, 7, CreateProjectInput{Name: "Telemetry A"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	environments, err := h.service.ListProjectEnvironments(ctx, project.ID)
	if err != nil || len(environments) != 1 {
		t.Fatalf("default environment: %v, %+v", err, environments)
	}
	issued, err := h.service.CreateTelemetryCredential(ctx, project.ID, 7, CreateTelemetryCredentialInput{
		Name: "production collector", EnvironmentID: environments[0].ID, ExpiresIn: time.Hour,
	})
	if err != nil {
		t.Fatalf("issue credential: %v", err)
	}
	if issued.Token == "" || issued.Credential.TokenPrefix == "" || len(issued.Credential.TokenHash) != 32 {
		t.Fatalf("expected opaque token and stored hash, got %+v", issued)
	}
	principal, err := h.service.AuthenticateTelemetryCredential(ctx, issued.Token)
	if err != nil {
		t.Fatalf("authenticate issued token: %v", err)
	}
	if principal.ProjectID != project.ID || principal.EnvironmentID != environments[0].ID {
		t.Fatalf("unexpected server-bound attribution: %+v", principal)
	}
	if principal.LastUsedAt == nil {
		t.Fatal("successful authentication must record use")
	}

	if err = h.service.RevokeTelemetryCredential(ctx, project.ID, issued.Credential.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err = h.service.AuthenticateTelemetryCredential(ctx, issued.Token); !errors.Is(err, ErrTelemetryCredentialNotFound) {
		t.Fatalf("revoked token must be rejected, got %v", err)
	}
}

func TestTelemetryCredentialRejectsForeignEnvironmentAndInvalidLifetime(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	projectA, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "A"})
	if err != nil {
		t.Fatalf("create A: %v", err)
	}
	projectB, err := h.service.CreateProject(ctx, 2, CreateProjectInput{Name: "B"})
	if err != nil {
		t.Fatalf("create B: %v", err)
	}
	foreign, err := h.service.CreateProjectEnvironment(ctx, projectB.ID, CreateEnvironmentInput{Name: "staging"})
	if err != nil {
		t.Fatalf("create foreign environment: %v", err)
	}
	if _, err = h.service.CreateTelemetryCredential(ctx, projectA.ID, 1, CreateTelemetryCredentialInput{Name: "bad", EnvironmentID: foreign.ID}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("foreign environment must be rejected, got %v", err)
	}
	local, _ := h.service.ListProjectEnvironments(ctx, projectA.ID)
	if _, err = h.service.CreateTelemetryCredential(ctx, projectA.ID, 1, CreateTelemetryCredentialInput{Name: "too long", EnvironmentID: local[0].ID, ExpiresIn: 366 * 24 * time.Hour}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("overlong lifetime must be rejected, got %v", err)
	}
}

func TestRotateTelemetryCredentialRevokesPriorSecret(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "Rotate"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	environments, _ := h.service.ListProjectEnvironments(ctx, project.ID)
	first, err := h.service.CreateTelemetryCredential(ctx, project.ID, 1, CreateTelemetryCredentialInput{Name: "collector", EnvironmentID: environments[0].ID})
	if err != nil {
		t.Fatalf("issue first: %v", err)
	}
	second, err := h.service.RotateTelemetryCredential(ctx, project.ID, 1, first.Credential.ID, CreateTelemetryCredentialInput{})
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if second.Credential.ID == first.Credential.ID || second.Token == first.Token {
		t.Fatalf("rotation must create a distinct credential: %+v", second)
	}
	if _, err = h.service.AuthenticateTelemetryCredential(ctx, first.Token); !errors.Is(err, ErrTelemetryCredentialNotFound) {
		t.Fatalf("old token must be revoked, got %v", err)
	}
	if _, err = h.service.AuthenticateTelemetryCredential(ctx, second.Token); err != nil {
		t.Fatalf("rotated token must authenticate: %v", err)
	}
}

func TestTelemetryIngressRecordsOnlyBoundedDiagnostics(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "Ingress diagnostics"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	environments, _ := h.service.ListProjectEnvironments(ctx, project.ID)
	issued, err := h.service.CreateTelemetryCredential(ctx, project.ID, 1, CreateTelemetryCredentialInput{Name: "collector", EnvironmentID: environments[0].ID})
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	principal, err := h.service.AuthenticateTelemetryCredential(ctx, issued.Token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	record := models.TelemetryIngressRecord{
		ProjectID: project.ID, EnvironmentID: environments[0].ID, CredentialID: principal.ID,
		SignalType: "metrics", ServiceName: "checkout", DeploymentEnvironment: "production", ItemCount: 4,
	}
	if err = h.service.RecordTelemetryIngress(ctx, principal, record); err != nil {
		t.Fatalf("record ingress: %v", err)
	}
	items, err := h.service.ListTelemetryIngress(ctx, project.ID, 10)
	if err != nil || len(items) != 1 {
		t.Fatalf("list ingress: %v %+v", err, items)
	}
	if items[0].ReceivedAt.IsZero() || items[0].ServiceName != "checkout" || items[0].ItemCount != 4 {
		t.Fatalf("unexpected diagnostics record: %+v", items[0])
	}
	for _, invalid := range []models.TelemetryIngressRecord{
		{ProjectID: project.ID + 1, EnvironmentID: environments[0].ID, CredentialID: principal.ID, SignalType: "metrics"},
		{ProjectID: project.ID, EnvironmentID: environments[0].ID, CredentialID: principal.ID, SignalType: "logs"},
		{ProjectID: project.ID, EnvironmentID: environments[0].ID, CredentialID: principal.ID, SignalType: "metrics", ItemCount: 10_001},
	} {
		if err = h.service.RecordTelemetryIngress(ctx, principal, invalid); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("invalid record must be rejected, got %v", err)
		}
	}
}
