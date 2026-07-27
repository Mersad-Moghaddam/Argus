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
	TypeEnqueueDueRoutes = "route:enqueue_due_checks"
	TypeCheckRoute       = "route:check"
	TypeAggregateRoutes  = "route:aggregate_metrics"
	TypePruneRouteChecks = "route:prune_checks"
)

type CheckWebsitePayload struct {
	WebsiteID      int64   `json:"websiteId"`
	URL            string  `json:"url"`
	HealthCheckURL *string `json:"healthCheckUrl,omitempty"`
	Interval       int     `json:"interval"`
}

type CheckRoutePayload struct {
	RouteID int64 `json:"routeId"`
}

func NewEnqueueDueChecksTask() *asynq.Task { return asynq.NewTask(TypeEnqueueDueChecks, nil) }
func NewDispatchOutboxTask() *asynq.Task   { return asynq.NewTask(TypeDispatchOutbox, nil) }
func NewEnqueueDueRoutesTask() *asynq.Task { return asynq.NewTask(TypeEnqueueDueRoutes, nil) }
func NewAggregateRoutesTask() *asynq.Task  { return asynq.NewTask(TypeAggregateRoutes, nil) }
func NewPruneRouteChecksTask() *asynq.Task { return asynq.NewTask(TypePruneRouteChecks, nil) }

func NewCheckWebsiteTask(payload CheckWebsitePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal check website payload: %w", err)
	}
	return asynq.NewTask(TypeCheckWebsite, body), nil
}

func NewCheckRouteTask(routeID int64) (*asynq.Task, error) {
	body, err := json.Marshal(CheckRoutePayload{RouteID: routeID})
	if err != nil {
		return nil, fmt.Errorf("marshal check route payload: %w", err)
	}
	return asynq.NewTask(TypeCheckRoute, body), nil
}
