package domain

import (
	"testing"
	"time"
)

func TestEvaluateSLOExplicitStatesAndBudgets(t *testing.T) {
	now := time.Now().UTC()
	base := SLOInput{Kind: SLIAvailability, TargetPercent: 99, Good: 100, Total: 100, ObservedAt: now, Now: now, StaleAfter: time.Minute}
	if got := EvaluateSLO(base); got.Status != SLOHealthy || got.ErrorBudgetRemaining != 100 {
		t.Fatalf("healthy=%+v", got)
	}
	base.Good = 98
	if got := EvaluateSLO(base); got.Status != SLOUnhealthy || got.BurnRate < 2 {
		t.Fatalf("unhealthy=%+v", got)
	}
	base.Total = 0
	base.Good = 0
	if got := EvaluateSLO(base); got.Status != SLONoData {
		t.Fatalf("no data=%+v", got)
	}
	base.Total = 100
	base.MinEvents = 101
	base.ObservedAt = now
	if got := EvaluateSLO(base); got.Status != SLONoData {
		t.Fatalf("low traffic=%+v", got)
	}
	base.MinEvents = 0
	base.ObservedAt = now.Add(-2 * time.Minute)
	if got := EvaluateSLO(base); got.Status != SLOStale {
		t.Fatalf("stale=%+v", got)
	}
	base.Paused = true
	if got := EvaluateSLO(base); got.Status != SLOPaused {
		t.Fatalf("paused=%+v", got)
	}
	base.Paused = false
	base.Maintenance = true
	if got := EvaluateSLO(base); got.Status != SLOMaintenance {
		t.Fatalf("maintenance=%+v", got)
	}
}
func TestMultiWindowBurnAlert(t *testing.T) {
	bad := SLOResult{Status: SLOUnhealthy, BurnRate: 10}
	if !MultiWindowBurnAlert(bad, bad, 8, 4) {
		t.Fatal("both windows over threshold must alert")
	}
	if MultiWindowBurnAlert(bad, SLOResult{Status: SLOHealthy, BurnRate: 10}, 8, 4) {
		t.Fatal("healthy long window must suppress alert")
	}
}
