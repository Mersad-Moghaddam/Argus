package worker

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"argus/internal/models"
	"github.com/hibiken/asynq"
)

// fakeRouteStore is a hand-rolled in-memory RouteStore. Only the methods the
// worker exercises carry behaviour; the rest satisfy the port.
type fakeRouteStore struct {
	mu             sync.Mutex
	routes         []models.APIRoute
	listDueCalls   int
	aggregateSince []time.Time
	aggregateProj  int
	pruneCalls     []time.Time
	pruneRemaining int64
	pruneErr       error
	listDueErr     error
}

func (f *fakeRouteStore) ListDueRoutes(_ context.Context, now time.Time, limit int, afterID int64) ([]models.APIRoute, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listDueCalls++
	if f.listDueErr != nil {
		return nil, f.listDueErr
	}
	out := []models.APIRoute{}
	for _, r := range f.routes {
		if r.ID <= afterID {
			continue
		}
		// Mirrors the SQL predicate: enabled AND due.
		if !r.Enabled || r.NextCheckAt.After(now) {
			continue
		}
		out = append(out, r)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeRouteStore) GetRouteByID(_ context.Context, id int64) (*models.APIRoute, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := range f.routes {
		if f.routes[i].ID == id {
			r := f.routes[i]
			return &r, nil
		}
	}
	return nil, nil
}

func (f *fakeRouteStore) AggregateRouteMetrics(_ context.Context, since time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.aggregateSince = append(f.aggregateSince, since)
	return nil
}

func (f *fakeRouteStore) AggregateProjectMetrics(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.aggregateProj++
	return nil
}

func (f *fakeRouteStore) PruneRouteChecks(_ context.Context, before time.Time, batchSize int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pruneCalls = append(f.pruneCalls, before)
	if f.pruneErr != nil {
		return 0, f.pruneErr
	}
	deleted := int64(batchSize)
	if f.pruneRemaining < deleted {
		deleted = f.pruneRemaining
	}
	f.pruneRemaining -= deleted
	return deleted, nil
}

// Unused-by-the-worker port methods.
func (f *fakeRouteStore) CreateRoute(context.Context, models.APIRoute) (int64, error) { return 0, nil }
func (f *fakeRouteStore) BulkCreateRoutes(context.Context, []models.APIRoute) (int, error) {
	return 0, nil
}
func (f *fakeRouteStore) UpdateRoute(context.Context, models.APIRoute) error { return nil }
func (f *fakeRouteStore) UpdateRouteImportedMetadata(context.Context, models.APIRoute) error {
	return nil
}
func (f *fakeRouteStore) SetRouteEnabled(context.Context, int64, bool) error { return nil }
func (f *fakeRouteStore) DeleteRoute(context.Context, int64) error           { return nil }
func (f *fakeRouteStore) BulkDeleteRoutes(context.Context, int64, []int64) (int64, error) {
	return 0, nil
}
func (f *fakeRouteStore) GetRouteByMethodPath(context.Context, int64, string, string) (*models.APIRoute, error) {
	return nil, nil
}
func (f *fakeRouteStore) ListRoutes(context.Context, models.RouteFilter) ([]models.APIRoute, int, error) {
	return nil, 0, nil
}
func (f *fakeRouteStore) ListAllRouteKeys(context.Context, int64) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (f *fakeRouteStore) ListRouteSpecHashes(context.Context, int64) (map[int64]string, error) {
	return map[int64]string{}, nil
}
func (f *fakeRouteStore) MarkRouteChecked(context.Context, int64, string, int, int, string, int, int, string, time.Time, time.Time) error {
	return nil
}
func (f *fakeRouteStore) RecordRouteCheck(context.Context, models.RouteCheck) error { return nil }
func (f *fakeRouteStore) ListRouteChecks(context.Context, int64, int, int) ([]models.RouteCheck, error) {
	return nil, nil
}

// recordingEnqueuer captures enqueued tasks and can simulate asynq's
// duplicate-task rejection.
type recordingEnqueuer struct {
	mu        sync.Mutex
	tasks     []*asynq.Task
	seenUniq  map[int64]bool
	dedupe    bool
	failAfter int
	err       error
}

func (r *recordingEnqueuer) EnqueueContext(_ context.Context, task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil && len(r.tasks) >= r.failAfter {
		return nil, r.err
	}
	if r.dedupe {
		var p CheckRoutePayload
		_ = json.Unmarshal(task.Payload(), &p)
		if r.seenUniq == nil {
			r.seenUniq = map[int64]bool{}
		}
		if r.seenUniq[p.RouteID] {
			return nil, asynq.ErrDuplicateTask
		}
		r.seenUniq[p.RouteID] = true
	}
	r.tasks = append(r.tasks, task)
	return &asynq.TaskInfo{}, nil
}

func (r *recordingEnqueuer) routeIDs() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]int64, 0, len(r.tasks))
	for _, task := range r.tasks {
		var p CheckRoutePayload
		if err := json.Unmarshal(task.Payload(), &p); err == nil {
			ids = append(ids, p.RouteID)
		}
	}
	return ids
}

func dueRoute(id int64, enabled bool, dueAgo time.Duration) models.APIRoute {
	return models.APIRoute{
		ID: id, ProjectID: 7, Method: "GET", Path: "/r", BaseURL: "https://api.example.com",
		Enabled: enabled, MonitorIntervalSecs: 60, TimeoutMS: 2000,
		ExpectedStatusRange: "200-399", NextCheckAt: time.Now().UTC().Add(-dueAgo),
	}
}

func TestHandleEnqueueDueRouteChecksOnlyEnqueuesEnabledDueRoutes(t *testing.T) {
	store := &fakeRouteStore{routes: []models.APIRoute{
		dueRoute(1, true, time.Minute),
		dueRoute(2, false, time.Minute),  // disabled
		dueRoute(3, true, -time.Hour),    // not due yet
		dueRoute(4, true, 2*time.Minute), // due
	}}
	enqueuer := &recordingEnqueuer{}
	p := &Processor{routes: store, client: enqueuer}

	if err := p.HandleEnqueueDueRouteChecks(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := enqueuer.routeIDs()
	want := []int64{1, 4}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestHandleEnqueueDueRouteChecksPaginatesWithoutRepeating(t *testing.T) {
	routes := make([]models.APIRoute, 0, 25)
	for i := int64(1); i <= 25; i++ {
		routes = append(routes, dueRoute(i, true, time.Minute))
	}
	store := &fakeRouteStore{routes: routes}
	enqueuer := &recordingEnqueuer{}
	p := &Processor{routes: store, client: enqueuer, routeCfg: RouteMonitorConfig{DueBatchSize: 10}}

	if err := p.HandleEnqueueDueRouteChecks(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := enqueuer.routeIDs()
	if len(ids) != 25 {
		t.Fatalf("expected all 25 routes enqueued exactly once, got %d", len(ids))
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("route %d enqueued twice", id)
		}
		seen[id] = true
	}
	if store.listDueCalls != 3 {
		t.Fatalf("expected 3 cursor pages (10+10+5), got %d", store.listDueCalls)
	}
}

// TestHandleEnqueueDueRouteChecksToleratesDuplicates proves the duplicate-job
// guard: asynq rejects a route that is already queued and the scan continues
// rather than aborting the whole cycle.
func TestHandleEnqueueDueRouteChecksToleratesDuplicates(t *testing.T) {
	store := &fakeRouteStore{routes: []models.APIRoute{
		dueRoute(1, true, time.Minute),
		dueRoute(2, true, time.Minute),
	}}
	enqueuer := &recordingEnqueuer{dedupe: true, seenUniq: map[int64]bool{1: true}}
	p := &Processor{routes: store, client: enqueuer}

	if err := p.HandleEnqueueDueRouteChecks(context.Background(), nil); err != nil {
		t.Fatalf("duplicate tasks must not fail the cycle: %v", err)
	}
	ids := enqueuer.routeIDs()
	if len(ids) != 1 || ids[0] != 2 {
		t.Fatalf("expected only route 2 to be enqueued, got %v", ids)
	}
}

func TestHandleEnqueueDueRouteChecksPropagatesStoreError(t *testing.T) {
	store := &fakeRouteStore{listDueErr: errors.New("db down")}
	p := &Processor{routes: store, client: &recordingEnqueuer{}}
	if err := p.HandleEnqueueDueRouteChecks(context.Background(), nil); err == nil {
		t.Fatal("expected the store error to propagate so asynq retries the scan")
	}
}

func TestHandleAggregateRouteMetrics(t *testing.T) {
	store := &fakeRouteStore{}
	p := &Processor{routes: store, routeCfg: RouteMonitorConfig{AggregationWindow: 6 * time.Hour}}
	if err := p.HandleAggregateRouteMetrics(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.aggregateSince) != 1 {
		t.Fatalf("expected one route aggregation, got %d", len(store.aggregateSince))
	}
	window := time.Since(store.aggregateSince[0])
	if window < 6*time.Hour || window > 6*time.Hour+time.Minute {
		t.Fatalf("expected a ~6h look-back window, got %s", window)
	}
	if store.aggregateProj != 1 {
		t.Fatalf("expected project metrics to be rolled up once, got %d", store.aggregateProj)
	}
}

func TestHandlePruneRouteChecksDrainsInBoundedBatches(t *testing.T) {
	store := &fakeRouteStore{pruneRemaining: 250}
	p := &Processor{routes: store, routeCfg: RouteMonitorConfig{CheckRetention: 48 * time.Hour, PruneBatchSize: 100}}
	if err := p.HandlePruneRouteChecks(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.pruneCalls) != 3 {
		t.Fatalf("expected 3 bounded batches (100+100+50), got %d", len(store.pruneCalls))
	}
	if store.pruneRemaining != 0 {
		t.Fatalf("expected the backlog to be drained, %d left", store.pruneRemaining)
	}
	age := time.Since(store.pruneCalls[0])
	if age < 48*time.Hour || age > 48*time.Hour+time.Minute {
		t.Fatalf("expected a 48h retention cutoff, got %s", age)
	}
}

func TestHandlePruneRouteChecksStopsAtIterationCap(t *testing.T) {
	store := &fakeRouteStore{pruneRemaining: 1 << 40}
	p := &Processor{routes: store, routeCfg: RouteMonitorConfig{PruneBatchSize: 1}}
	if err := p.HandlePruneRouteChecks(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(store.pruneCalls) != maxPruneIterations {
		t.Fatalf("expected pruning to stop after %d iterations, got %d", maxPruneIterations, len(store.pruneCalls))
	}
}

func TestHandleCheckRouteSkipsMissingAndDisabledRoutes(t *testing.T) {
	store := &fakeRouteStore{routes: []models.APIRoute{dueRoute(2, false, time.Minute)}}
	p := &Processor{routes: store, evaluator: NewRouteEvaluator(EvaluatorConfig{})}

	missing, _ := NewCheckRouteTask(CheckRoutePayload{RouteID: 99})
	if err := p.HandleCheckRoute(context.Background(), missing); err != nil {
		t.Fatalf("a deleted route must not fail the task: %v", err)
	}
	disabled, _ := NewCheckRouteTask(CheckRoutePayload{RouteID: 2})
	if err := p.HandleCheckRoute(context.Background(), disabled); err != nil {
		t.Fatalf("a disabled route must not fail the task: %v", err)
	}
}

func TestHandleCheckRouteSkipsRetryOnMalformedPayload(t *testing.T) {
	p := &Processor{routes: &fakeRouteStore{}}
	err := p.HandleCheckRoute(context.Background(), asynq.NewTask(TypeCheckRoute, []byte("{not json")))
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected SkipRetry for an unparsable payload, got %v", err)
	}
}

func TestRegisterRouteTasksIsNoOpWithoutRouteStore(t *testing.T) {
	p := &Processor{}
	mux := asynq.NewServeMux()
	p.RegisterRouteTasks(mux) // must not panic or register anything
}
