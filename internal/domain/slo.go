package domain

import (
	"math"
	"time"
)

type SLOStatus string

const (
	SLOHealthy            SLOStatus = "healthy"
	SLOUnhealthy          SLOStatus = "unhealthy"
	SLONoData             SLOStatus = "no_data"
	SLOStale              SLOStatus = "stale"
	SLOPaused             SLOStatus = "paused"
	SLOMaintenance        SLOStatus = "maintenance"
	SLOConfigurationError SLOStatus = "configuration_error"
)

type SLIKind string

const (
	SLIAvailability SLIKind = "availability"
	SLILatency      SLIKind = "latency"
)

type SLOInput struct {
	Kind                SLIKind
	TargetPercent       float64
	LatencyThresholdMS  float64
	Good, Total         int64
	ObservedAt          time.Time
	Now                 time.Time
	StaleAfter          time.Duration
	Paused, Maintenance bool
}
type SLOResult struct {
	Status                                          SLOStatus
	ObservedPercent, ErrorBudgetRemaining, BurnRate float64
}

// EvaluateSLO never treats absent or delayed telemetry as success. Callers
// provide already-scoped aggregates from the time-series backend.
func EvaluateSLO(in SLOInput) SLOResult {
	if in.Paused {
		return SLOResult{Status: SLOPaused}
	}
	if in.Maintenance {
		return SLOResult{Status: SLOMaintenance}
	}
	if in.Kind != SLIAvailability && in.Kind != SLILatency || in.TargetPercent <= 0 || in.TargetPercent >= 100 || in.Total < 0 || in.Good < 0 || in.Good > in.Total || in.StaleAfter <= 0 {
		return SLOResult{Status: SLOConfigurationError}
	}
	if in.Total == 0 || in.ObservedAt.IsZero() {
		return SLOResult{Status: SLONoData}
	}
	if in.Now.Sub(in.ObservedAt) > in.StaleAfter {
		return SLOResult{Status: SLOStale}
	}
	observed := float64(in.Good) * 100 / float64(in.Total)
	allowed := 100 - in.TargetPercent
	badPercent := 100 - observed
	burn := badPercent / allowed
	budget := (allowed - badPercent) / allowed * 100
	if budget < 0 {
		budget = 0
	}
	if budget > 100 {
		budget = 100
	}
	status := SLOHealthy
	if observed < in.TargetPercent {
		status = SLOUnhealthy
	}
	return SLOResult{Status: status, ObservedPercent: observed, ErrorBudgetRemaining: budget, BurnRate: math.Max(0, burn)}
}

// MultiWindowBurnAlert requires both the short and long observation windows to
// be above their configured burn thresholds, avoiding one-window alert noise.
func MultiWindowBurnAlert(short, long SLOResult, shortThreshold, longThreshold float64) bool {
	return short.Status == SLOUnhealthy && long.Status == SLOUnhealthy && short.BurnRate >= shortThreshold && long.BurnRate >= longThreshold
}
