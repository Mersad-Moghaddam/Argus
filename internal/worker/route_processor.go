package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
)

// RouteMonitorConfig tunes the background route-monitoring tasks.
type RouteMonitorConfig struct {
	// DueBatchSize is how many due routes are fetched per cursor page.
	DueBatchSize int
	// CheckRetention is how long raw route_checks rows are kept.
	CheckRetention time.Duration
	// PruneBatchSize bounds each DELETE so pruning never locks the table
	// for long on projects with high-volume time-series data.
	PruneBatchSize int
	// AggregationWindow is the look-back used for the cached 24h metrics.
	AggregationWindow time.Duration
	// ProjectDailyBudget and GlobalDailyBudget are durable UTC-day request
	// ceilings. They protect targets and the control plane before a task enters
	// the queue; zero or negative values use the conservative defaults.
	ProjectDailyBudget int
	GlobalDailyBudget  int
	// ProjectConcurrency and GlobalConcurrency cap actual in-flight HTTP
	// executions; queue depth alone is not a safe proxy for outbound pressure.
	ProjectConcurrency int
	GlobalConcurrency  int
}

func (c RouteMonitorConfig) withDefaults() RouteMonitorConfig {
	if c.DueBatchSize <= 0 || c.DueBatchSize > 1000 {
		c.DueBatchSize = 200
	}
	if c.CheckRetention <= 0 {
		c.CheckRetention = 30 * 24 * time.Hour
	}
	if c.PruneBatchSize <= 0 || c.PruneBatchSize > 50000 {
		c.PruneBatchSize = 5000
	}
	if c.AggregationWindow <= 0 {
		c.AggregationWindow = 24 * time.Hour
	}
	if c.ProjectDailyBudget <= 0 {
		c.ProjectDailyBudget = 10000
	}
	if c.GlobalDailyBudget <= 0 {
		c.GlobalDailyBudget = 100000
	}
	if c.ProjectConcurrency <= 0 {
		c.ProjectConcurrency = 4
	}
	if c.GlobalConcurrency <= 0 {
		c.GlobalConcurrency = 50
	}
	return c
}

// maxPruneIterations bounds one pruning run so a large backlog is drained
// across several scheduled runs instead of holding a worker indefinitely.
const maxPruneIterations = 40

// RegisterRouteTasks wires the project route-monitoring handlers. It is a
// no-op when the processor was built without a route store, so the legacy
// website-only worker keeps working unchanged.
func (p *Processor) RegisterRouteTasks(mux *asynq.ServeMux) {
	if p.routes == nil {
		return
	}
	mux.HandleFunc(TypeEnqueueDueRouteChecks, p.HandleEnqueueDueRouteChecks)
	mux.HandleFunc(TypeCheckRoute, p.HandleCheckRoute)
	mux.HandleFunc(TypeAggregateRouteMetrics, p.HandleAggregateRouteMetrics)
	mux.HandleFunc(TypePruneRouteChecks, p.HandlePruneRouteChecks)
}

// HandleEnqueueDueRouteChecks scans enabled routes whose next check is due
// and fans out one check task each. Pagination is keyed on the route ID
// cursor so the scan stays O(page) regardless of how many thousands of routes
// a project holds. asynq's uniqueness constraint (keyed on task type +
// payload) guarantees a route that is already queued or in flight is never
// enqueued twice within its own check interval.
func (p *Processor) HandleEnqueueDueRouteChecks(ctx context.Context, _ *asynq.Task) error {
	cfg := p.routeCfg.withDefaults()
	now := time.Now().UTC()
	afterID := int64(0)
	enqueued := 0
	for {
		due, err := p.routes.ListDueRoutes(ctx, now, cfg.DueBatchSize, afterID)
		if err != nil {
			return err
		}
		if len(due) == 0 {
			break
		}
		for _, route := range due {
			afterID = route.ID
			if !route.Enabled {
				continue
			}
			requests := route.Retries + 1
			if requests > MaxRouteRetries+1 {
				requests = MaxRouteRetries + 1
			}
			reserved, reason, reserveErr := p.routes.ReserveSyntheticBudget(ctx, route.ProjectID, now, requests, cfg.ProjectDailyBudget, cfg.GlobalDailyBudget)
			if reserveErr != nil {
				return reserveErr
			}
			if !reserved {
				next := now.Add(time.Duration(route.MonitorIntervalSecs) * time.Second)
				if err := p.routes.DeferRouteCheck(ctx, route.ID, next); err != nil {
					return err
				}
				if err := p.routes.RecordSyntheticSkip(ctx, route.ID, route.ProjectID, reason, now); err != nil {
					return err
				}
				if p.logger != nil {
					p.logger.Add("info", "worker", "route_check_shed", "Skipped synthetic route check because its request budget is exhausted", nil, map[string]string{"project_id": strconv.FormatInt(route.ProjectID, 10), "route_id": strconv.FormatInt(route.ID, 10), "reason": reason})
				}
				continue
			}
			task, taskErr := NewCheckRouteTask(CheckRoutePayload{RouteID: route.ID, ProjectID: route.ProjectID, Interval: route.MonitorIntervalSecs})
			if taskErr != nil {
				return taskErr
			}
			uniqueFor := time.Duration(route.MonitorIntervalSecs) * time.Second
			if uniqueFor < 10*time.Second {
				uniqueFor = 10 * time.Second
			}
			_, enqueueErr := p.client.EnqueueContext(ctx, task,
				asynq.Queue("default"),
				asynq.ProcessIn(syntheticJitter(route.ID, route.MonitorIntervalSecs)),
				asynq.Unique(uniqueFor),
				asynq.MaxRetry(2),
				asynq.Timeout(MaxRouteTimeout*time.Duration(MaxRouteRetries+2)),
			)
			if enqueueErr != nil {
				if refundErr := p.routes.ReleaseSyntheticBudget(ctx, route.ProjectID, now, requests); refundErr != nil {
					return refundErr
				}
				if errors.Is(enqueueErr, asynq.ErrDuplicateTask) {
					continue
				}
				return enqueueErr
			}
			enqueued++
		}
		if len(due) < cfg.DueBatchSize {
			break
		}
	}
	if enqueued > 0 && p.logger != nil {
		p.logger.Add("debug", "worker", "route_checks_enqueued", "Enqueued due route checks", nil, map[string]string{"count": strconv.Itoa(enqueued)})
	}
	return nil
}

// syntheticJitter spreads due checks deterministically over a small fraction
// of their interval. It avoids synchronized bursts without adding randomness
// that would make scheduler behaviour hard to test or reproduce.
func syntheticJitter(routeID int64, intervalSeconds int) time.Duration {
	if intervalSeconds <= 1 {
		return 0
	}
	maxSeconds := intervalSeconds / 10
	if maxSeconds < 1 {
		maxSeconds = 1
	}
	if maxSeconds > 30 {
		maxSeconds = 30
	}
	return time.Duration(routeID%int64(maxSeconds+1)) * time.Second
}

// HandleCheckRoute performs one monitored request and hands the outcome to
// the application service, which is the single place that turns a raw result
// into route health, a persisted check row and incident transitions.
func (p *Processor) HandleCheckRoute(ctx context.Context, task *asynq.Task) error {
	var payload CheckRoutePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		// A malformed payload can never succeed on retry.
		return asynq.SkipRetry
	}
	route, err := p.routes.GetRouteByID(ctx, payload.RouteID)
	if err != nil {
		return err
	}
	if route == nil {
		// The route was deleted between enqueue and execution.
		return nil
	}
	if !route.Enabled {
		return nil
	}
	cfg := p.routeCfg.withDefaults()
	leaseKey := "route:" + strconv.FormatInt(route.ID, 10)
	attempts := route.Retries
	if attempts < 0 {
		attempts = 0
	}
	if attempts > MaxRouteRetries {
		attempts = MaxRouteRetries
	}
	leaseFor := time.Duration(route.TimeoutMS)*time.Millisecond*time.Duration(attempts+1) + 30*time.Second
	if leaseFor > MaxRouteTimeout*time.Duration(MaxRouteRetries+1)+30*time.Second {
		leaseFor = MaxRouteTimeout*time.Duration(MaxRouteRetries+1) + 30*time.Second
	}
	now := time.Now().UTC()
	leased, reason, leaseErr := p.routes.AcquireSyntheticLease(ctx, route.ProjectID, leaseKey, now, now.Add(leaseFor), cfg.ProjectConcurrency, cfg.GlobalConcurrency)
	if leaseErr != nil {
		return leaseErr
	}
	if !leased {
		next := now.Add(time.Duration(route.MonitorIntervalSecs) * time.Second)
		if err := p.routes.DeferRouteCheck(ctx, route.ID, next); err != nil {
			return err
		}
		if err := p.routes.RecordSyntheticSkip(ctx, route.ID, route.ProjectID, reason, now); err != nil {
			return err
		}
		if p.logger != nil {
			p.logger.Add("info", "worker", "route_check_shed", "Skipped synthetic route check because its concurrency cap is exhausted", nil, map[string]string{"project_id": strconv.FormatInt(route.ProjectID, 10), "route_id": strconv.FormatInt(route.ID, 10), "reason": reason})
		}
		return nil
	}
	defer func() { _ = p.routes.ReleaseSyntheticLease(context.Background(), leaseKey) }()
	outcome := p.evaluator.EvaluateRoute(ctx, *route)
	return p.service.ProcessRouteCheckResult(ctx, *route, outcome.Status, outcome.StatusCode, outcome.LatencyMS, outcome.FailureReason, outcome.Attempts, time.Now().UTC())
}

// HandleAggregateRouteMetrics refreshes the cached rolling-window metrics on
// api_routes and projects with two batched set-based statements, so dashboard
// reads never scan the raw time-series table.
func (p *Processor) HandleAggregateRouteMetrics(ctx context.Context, _ *asynq.Task) error {
	cfg := p.routeCfg.withDefaults()
	since := time.Now().UTC().Add(-cfg.AggregationWindow)
	if err := p.routes.AggregateRouteMetrics(ctx, since); err != nil {
		return err
	}
	return p.routes.AggregateProjectMetrics(ctx)
}

// HandlePruneRouteChecks enforces the check-history retention window in
// bounded batches.
func (p *Processor) HandlePruneRouteChecks(ctx context.Context, _ *asynq.Task) error {
	cfg := p.routeCfg.withDefaults()
	before := time.Now().UTC().Add(-cfg.CheckRetention)
	total := int64(0)
	for i := 0; i < maxPruneIterations; i++ {
		deleted, err := p.routes.PruneRouteChecks(ctx, before, cfg.PruneBatchSize)
		if err != nil {
			return err
		}
		total += deleted
		if deleted < int64(cfg.PruneBatchSize) {
			break
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	if total > 0 && p.logger != nil {
		p.logger.Add("info", "worker", "route_checks_pruned", "Pruned expired route checks", nil, map[string]string{
			"deleted": strconv.FormatInt(total, 10),
			"before":  before.Format(time.RFC3339),
		})
	}
	return nil
}
