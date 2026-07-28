package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
)

func TestSLODefinitionAndEvaluationAreProjectScopedAndVersioned(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "SLO project"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	definition, err := h.service.CreateSLODefinition(ctx, project.ID, 1, CreateSLODefinitionInput{Name: "Checkout availability", SLIKind: string(domain.SLIAvailability), TargetPercent: 99.9, MinEvents: 100})
	if err != nil {
		t.Fatalf("create SLO: %v", err)
	}
	if definition.Version != 1 || definition.WindowSeconds != defaultSLOWindowSeconds || definition.ShortBurnRate != defaultSLOShortBurnRate {
		t.Fatalf("expected persisted policy defaults and initial version: %+v", definition)
	}
	items, err := h.service.ListSLODefinitions(ctx, project.ID)
	if err != nil || len(items) != 1 || items[0].ID != definition.ID {
		t.Fatalf("list definitions: %#v, %v", items, err)
	}
	now := time.Now().UTC()
	observed, budget, burn := 99.95, 50.0, 0.5
	evaluation, err := h.service.RecordSLOEvaluation(ctx, project.ID, definition.ID, models.SLOEvaluation{Status: string(domain.SLOHealthy), ObservedPercent: &observed, ErrorBudgetRemaining: &budget, BurnRate: &burn, GoodEvents: 10_000, TotalEvents: 10_005, WindowStartedAt: now.Add(-time.Hour), WindowEndedAt: now, ObservedAt: &now, Provenance: "victoriametrics/http-server"})
	if err != nil {
		t.Fatalf("record evaluation: %v", err)
	}
	if evaluation.DefinitionVersion != definition.Version || evaluation.ID == 0 {
		t.Fatalf("evaluation must retain definition version: %+v", evaluation)
	}
	evaluations, err := h.service.ListSLOEvaluations(ctx, project.ID, definition.ID, 10)
	if err != nil || len(evaluations) != 1 || evaluations[0].Provenance != "victoriametrics/http-server" {
		t.Fatalf("list evaluations: %#v, %v", evaluations, err)
	}
	other, err := h.service.CreateProject(ctx, 2, CreateProjectInput{Name: "Other SLO project"})
	if err != nil {
		t.Fatalf("create other project: %v", err)
	}
	if _, err = h.service.ListSLOEvaluations(ctx, other.ID, definition.ID, 10); !errors.Is(err, ErrSLODefinitionNotFound) {
		t.Fatalf("cross-project evaluation list must be hidden, got %v", err)
	}
}

func TestSLODefinitionValidatesLatencyAndBurnWindows(t *testing.T) {
	h := newTestHarness()
	project, err := h.service.CreateProject(context.Background(), 1, CreateProjectInput{Name: "Validation"})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []CreateSLODefinitionInput{
		{Name: "bad kind", SLIKind: "errors", TargetPercent: 99},
		{Name: "bad latency", SLIKind: string(domain.SLILatency), TargetPercent: 99},
		{Name: "bad availability", SLIKind: string(domain.SLIAvailability), TargetPercent: 99, LatencyThresholdMS: 100},
		{Name: "bad windows", SLIKind: string(domain.SLIAvailability), TargetPercent: 99, WindowSeconds: 3600, ShortWindowSeconds: 3500, LongWindowSeconds: 3400},
	} {
		if _, err := h.service.CreateSLODefinition(context.Background(), project.ID, 1, input); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("input %+v must fail, got %v", input, err)
		}
	}
}
