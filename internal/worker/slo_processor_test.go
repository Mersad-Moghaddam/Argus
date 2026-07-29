package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"argus/internal/models"
	"argus/internal/testsupport"
)

type fixedSLOMetricsReader struct {
	aggregate models.SLOMetricAggregate
	err       error
}

func (r fixedSLOMetricsReader) AggregateSLO(_ context.Context, _ models.SLODefinition, _ time.Time) (models.SLOMetricAggregate, error) {
	return r.aggregate, r.err
}

func TestSLOEvaluatorRecordsHealthyAndNoDataEvidence(t *testing.T) {
	store := testsupport.NewSLOStore()
	definition := models.SLODefinition{ProjectID: 8, CreatedByUserID: 1, Name: "Availability", SLIKind: "availability", TargetPercent: 99, WindowSeconds: 3600, ShortWindowSeconds: 300, LongWindowSeconds: 900, ShortBurnRate: 14.4, LongBurnRate: 6}
	id, err := store.CreateSLODefinition(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	definition.ID, definition.Version = id, 1
	now := time.Now().UTC()
	evaluator := NewSLOEvaluator(store, fixedSLOMetricsReader{aggregate: models.SLOMetricAggregate{GoodEvents: 100, TotalEvents: 100, ObservedAt: &now, Provenance: "test"}}, time.Minute)
	if err = evaluator.EvaluateDefinition(context.Background(), definition, now); err != nil {
		t.Fatal(err)
	}
	items, err := store.ListSLOEvaluations(context.Background(), definition.ProjectID, definition.ID, 10)
	if err != nil || len(items) != 1 || items[0].Status != "healthy" || items[0].ObservedPercent == nil {
		t.Fatalf("healthy evidence: %#v, %v", items, err)
	}
	evaluator = NewSLOEvaluator(store, fixedSLOMetricsReader{aggregate: models.SLOMetricAggregate{TotalEvents: 5, GoodEvents: 5, ObservedAt: &now, Provenance: "test"}}, time.Minute)
	definition.MinEvents = 10
	if err = evaluator.EvaluateDefinition(context.Background(), definition, now); err != nil {
		t.Fatal(err)
	}
	items, _ = store.ListSLOEvaluations(context.Background(), definition.ProjectID, definition.ID, 10)
	if items[0].Status != "no_data" || items[0].ObservedPercent != nil {
		t.Fatalf("low traffic must have no numeric SLO result: %+v", items[0])
	}
}

func TestSLOEvaluatorRecordsConfigurationErrorForMetricsFailure(t *testing.T) {
	store := testsupport.NewSLOStore()
	definition := models.SLODefinition{ProjectID: 9, CreatedByUserID: 1, Name: "Metrics error", SLIKind: "availability", TargetPercent: 99, WindowSeconds: 3600, ShortWindowSeconds: 300, LongWindowSeconds: 900, ShortBurnRate: 14.4, LongBurnRate: 6}
	id, err := store.CreateSLODefinition(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	definition.ID, definition.Version = id, 1
	if err = NewSLOEvaluator(store, fixedSLOMetricsReader{err: errors.New("backend unavailable")}, time.Minute).EvaluateDefinition(context.Background(), definition, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	items, _ := store.ListSLOEvaluations(context.Background(), definition.ProjectID, definition.ID, 10)
	if len(items) != 1 || items[0].Status != "configuration_error" {
		t.Fatalf("unexpected metrics failure evidence: %#v", items)
	}
}

func TestSLOEvaluatorEnqueuesOnlyStateTransitions(t *testing.T) {
	store := testsupport.NewSLOStore()
	outbox := &testsupport.OutboxStore{}
	definition := models.SLODefinition{ProjectID: 10, CreatedByUserID: 1, Name: "Availability", SLIKind: "availability", TargetPercent: 99, WindowSeconds: 3600, ShortWindowSeconds: 300, LongWindowSeconds: 900, ShortBurnRate: 14.4, LongBurnRate: 6}
	id, err := store.CreateSLODefinition(context.Background(), definition)
	if err != nil {
		t.Fatal(err)
	}
	definition.ID, definition.Version = id, 1
	now := time.Now().UTC()
	evaluator := NewSLOEvaluator(store, fixedSLOMetricsReader{aggregate: models.SLOMetricAggregate{GoodEvents: 90, TotalEvents: 100, ObservedAt: &now, Provenance: "test"}}, time.Minute, outbox)
	if err = evaluator.EvaluateDefinition(context.Background(), definition, now); err != nil {
		t.Fatal(err)
	}
	if got := outbox.EventTypes(); len(got) != 1 || got[0] != "slo_unhealthy" {
		t.Fatalf("unhealthy transition: %v", got)
	}
	if err = evaluator.EvaluateDefinition(context.Background(), definition, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := outbox.EventTypes(); len(got) != 1 {
		t.Fatalf("duplicate state emitted: %v", got)
	}
	recoveredAt := now.Add(2 * time.Minute)
	evaluator = NewSLOEvaluator(store, fixedSLOMetricsReader{aggregate: models.SLOMetricAggregate{GoodEvents: 100, TotalEvents: 100, ObservedAt: &recoveredAt, Provenance: "test"}}, time.Minute, outbox)
	if err = evaluator.EvaluateDefinition(context.Background(), definition, recoveredAt); err != nil {
		t.Fatal(err)
	}
	if got := outbox.EventTypes(); len(got) != 2 || got[1] != "slo_recovered" {
		t.Fatalf("recovery transition: %v", got)
	}
}
