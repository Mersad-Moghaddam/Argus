package worker

import (
	"context"
	"time"

	"argus/internal/domain"
	"argus/internal/domain/ports"
	"argus/internal/models"
	"github.com/hibiken/asynq"
)

const defaultSLOStaleAfter = 10 * time.Minute

// SLOEvaluator turns safe metrics aggregates into immutable control-plane
// evidence. It is separate from the HTTP API so no client can forge results.
type SLOEvaluator struct {
	store      ports.SLOStore
	metrics    ports.SLOMetricsReader
	staleAfter time.Duration
}

func NewSLOEvaluator(store ports.SLOStore, metrics ports.SLOMetricsReader, staleAfter time.Duration) *SLOEvaluator {
	if staleAfter <= 0 {
		staleAfter = defaultSLOStaleAfter
	}
	return &SLOEvaluator{store: store, metrics: metrics, staleAfter: staleAfter}
}

func (e *SLOEvaluator) EvaluateAll(ctx context.Context, now time.Time) error {
	for afterID := int64(0); ; {
		definitions, err := e.store.ListSLODefinitionsForEvaluation(ctx, 100, afterID)
		if err != nil {
			return err
		}
		for _, definition := range definitions {
			afterID = definition.ID
			if err = e.EvaluateDefinition(ctx, definition, now); err != nil {
				return err
			}
		}
		if len(definitions) < 100 {
			return nil
		}
	}
}

func (e *SLOEvaluator) EvaluateDefinition(ctx context.Context, definition models.SLODefinition, now time.Time) error {
	aggregate, err := e.metrics.AggregateSLO(ctx, definition, now)
	if err != nil {
		return e.record(ctx, definition, now, models.SLOMetricAggregate{Provenance: "victoriametrics/http-server"}, domain.SLOResult{Status: domain.SLOConfigurationError})
	}
	observedAt := time.Time{}
	if aggregate.ObservedAt != nil {
		observedAt = *aggregate.ObservedAt
	}
	result := domain.EvaluateSLO(domain.SLOInput{Kind: domain.SLIKind(definition.SLIKind), TargetPercent: definition.TargetPercent, LatencyThresholdMS: float64(definition.LatencyThresholdMS), Good: aggregate.GoodEvents, Total: aggregate.TotalEvents, MinEvents: int64(definition.MinEvents), ObservedAt: observedAt, Now: now, StaleAfter: e.staleAfter, Paused: definition.Paused})
	return e.record(ctx, definition, now, aggregate, result)
}

func (e *SLOEvaluator) record(ctx context.Context, definition models.SLODefinition, now time.Time, aggregate models.SLOMetricAggregate, result domain.SLOResult) error {
	evaluation := models.SLOEvaluation{SLOID: definition.ID, ProjectID: definition.ProjectID, DefinitionVersion: definition.Version, Status: string(result.Status), GoodEvents: aggregate.GoodEvents, TotalEvents: aggregate.TotalEvents, WindowStartedAt: now.Add(-time.Duration(definition.WindowSeconds) * time.Second), WindowEndedAt: now, ObservedAt: aggregate.ObservedAt, Provenance: aggregate.Provenance, EvaluatedAt: now}
	if evaluation.Provenance == "" {
		evaluation.Provenance = "victoriametrics/http-server"
	}
	if result.Status == domain.SLOHealthy || result.Status == domain.SLOUnhealthy {
		evaluation.ObservedPercent = float64Pointer(result.ObservedPercent)
		evaluation.ErrorBudgetRemaining = float64Pointer(result.ErrorBudgetRemaining)
		evaluation.BurnRate = float64Pointer(result.BurnRate)
	}
	_, err := e.store.RecordSLOEvaluation(ctx, evaluation)
	return err
}

func float64Pointer(value float64) *float64 { return &value }

func (p *Processor) SetSLOEvaluator(evaluator *SLOEvaluator) { p.sloEvaluator = evaluator }
func (p *Processor) RegisterSLOTasks(mux *asynq.ServeMux) {
	if p.sloEvaluator != nil {
		mux.HandleFunc(TypeEvaluateSLOs, p.HandleEvaluateSLOs)
	}
}
func (p *Processor) HandleEvaluateSLOs(ctx context.Context, _ *asynq.Task) error {
	if p.sloEvaluator == nil {
		return nil
	}
	return p.sloEvaluator.EvaluateAll(ctx, time.Now().UTC())
}
