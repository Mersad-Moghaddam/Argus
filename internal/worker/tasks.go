package worker

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const (
	TypeEnqueueDueChecks = "website:enqueue_due_checks"
	TypeCheckWebsite     = "website:check"
	TypeDispatchOutbox   = "outbox:dispatch"

	// Project-based API route monitoring tasks.
	TypeEnqueueDueRouteChecks     = "route:enqueue_due_checks"
	TypeCheckRoute                = "route:check"
	TypeAggregateRouteMetrics     = "route:aggregate_metrics"
	TypePruneRouteChecks          = "route:prune_checks"
	TypeEvaluateSLOs              = "slo:evaluate"
	TypeEvaluateAgentLiveness     = "agent:evaluate_liveness"
	TypeEvaluateHeartbeatLiveness = "heartbeat:evaluate_liveness"
)

type CheckWebsitePayload struct {
	WebsiteID      int64   `json:"websiteId"`
	URL            string  `json:"url"`
	HealthCheckURL *string `json:"healthCheckUrl,omitempty"`
	Interval       int     `json:"interval"`
}

// CheckRoutePayload identifies the route to check. Only the ID is carried:
// the handler re-reads the route so a check never acts on stale monitoring
// configuration that changed between enqueue and execution.
type CheckRoutePayload struct {
	RouteID   int64 `json:"routeId"`
	ProjectID int64 `json:"projectId"`
	Interval  int   `json:"interval"`
}

func NewEnqueueDueChecksTask() *asynq.Task { return asynq.NewTask(TypeEnqueueDueChecks, nil) }
func NewDispatchOutboxTask() *asynq.Task   { return asynq.NewTask(TypeDispatchOutbox, nil) }

func NewEnqueueDueRouteChecksTask() *asynq.Task {
	return asynq.NewTask(TypeEnqueueDueRouteChecks, nil)
}
func NewAggregateRouteMetricsTask() *asynq.Task {
	return asynq.NewTask(TypeAggregateRouteMetrics, nil)
}
func NewPruneRouteChecksTask() *asynq.Task      { return asynq.NewTask(TypePruneRouteChecks, nil) }
func NewEvaluateSLOsTask() *asynq.Task          { return asynq.NewTask(TypeEvaluateSLOs, nil) }
func NewEvaluateAgentLivenessTask() *asynq.Task { return asynq.NewTask(TypeEvaluateAgentLiveness, nil) }
func NewEvaluateHeartbeatLivenessTask() *asynq.Task {
	return asynq.NewTask(TypeEvaluateHeartbeatLiveness, nil)
}

func NewCheckWebsiteTask(payload CheckWebsitePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal check website payload: %w", err)
	}
	return asynq.NewTask(TypeCheckWebsite, body), nil
}

func NewCheckRouteTask(payload CheckRoutePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal check route payload: %w", err)
	}
	return asynq.NewTask(TypeCheckRoute, body), nil
}
