package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
)

func seedProject(t *testing.T, h *testHarness) models.Project {
	t.Helper()
	project, err := h.service.CreateProject(context.Background(), 1, CreateProjectInput{
		Name: "Test API", DefaultIntervalSeconds: 60, DefaultTimeoutMS: 3000, DefaultRetries: 1,
		FailureThreshold: 3, RecoverySuccessThreshold: 1,
	})
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	return project
}

func TestCreateRouteNormalizesAndInheritsProjectDefaults(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	route, err := h.service.CreateRoute(ctx, project, RouteInput{
		Method: " get ", Path: "v1/pets/", BaseURL: "https://api.example.com/",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if route.Method != "GET" {
		t.Fatalf("method not normalized: %q", route.Method)
	}
	if route.Path != "/v1/pets" {
		t.Fatalf("path not normalized: %q", route.Path)
	}
	if route.BaseURL != "https://api.example.com" {
		t.Fatalf("base URL not normalized: %q", route.BaseURL)
	}
	if route.CanonicalIdentity != "GET https://api.example.com/v1/pets" || len(route.CanonicalHash) != 32 || route.CanonicalVersion != 1 {
		t.Fatalf("canonical dual-write fields missing: %+v", route)
	}
	if route.MonitorIntervalSecs != 60 || route.TimeoutMS != 3000 || route.Retries != 1 {
		t.Fatalf("project monitoring defaults not inherited: %+v", route)
	}
	if route.ExpectedStatusRange != "200-399" {
		t.Fatalf("expected the default status range, got %q", route.ExpectedStatusRange)
	}
	if route.Status != domain.RouteStatusUnknown {
		t.Fatalf("a never-checked route must be unknown, got %q", route.Status)
	}
	if route.Source != "manual" {
		t.Fatalf("expected source=manual, got %q", route.Source)
	}
}

func TestCreateRouteRejectsInvalidAndDuplicate(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	if _, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "FETCH", Path: "/x", BaseURL: "https://a.example"}); !errors.Is(err, domain.ErrInvalidRoute) {
		t.Fatalf("expected ErrInvalidRoute for an unsupported method, got %v", err)
	}
	if _, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "   ", BaseURL: "https://a.example"}); !errors.Is(err, domain.ErrInvalidRoute) {
		t.Fatalf("expected ErrInvalidRoute for an empty path, got %v", err)
	}

	if _, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/dup", BaseURL: "https://a.example"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Same operation expressed differently must still collide.
	if _, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "get", Path: "dup/", BaseURL: "https://a.example"}); !errors.Is(err, domain.ErrDuplicateRoute) {
		t.Fatalf("expected ErrDuplicateRoute, got %v", err)
	}
}

func TestRoutesAreCatalogOnlyUntilAnExplicitSafeCanaryIsEnabled(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	catalog, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "POST", Path: "/orders", BaseURL: "https://api.example.com", Enabled: boolPtr(false)})
	if err != nil {
		t.Fatalf("create catalog entry: %v", err)
	}
	if catalog.Enabled || catalog.Status != domain.RouteStatusDisabled {
		t.Fatalf("a catalog entry must not start a synthetic check: %+v", catalog)
	}
	if _, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "DELETE", Path: "/orders/{id}", BaseURL: "https://api.example.com", Enabled: boolPtr(true)}); !errors.Is(err, domain.ErrUnsafeSynthetic) {
		t.Fatalf("expected unsafe enabled method to be rejected, got %v", err)
	}
	if err := h.service.SetRouteEnabled(ctx, catalog.ID, true); !errors.Is(err, domain.ErrUnsafeSynthetic) {
		t.Fatalf("expected unsafe catalog entry enablement to be rejected, got %v", err)
	}

	safe, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/health", BaseURL: "https://api.example.com", Enabled: boolPtr(true)})
	if err != nil {
		t.Fatalf("create safe canary: %v", err)
	}
	if !safe.Enabled {
		t.Fatal("explicit GET canary should be enabled")
	}
}

func boolPtr(v bool) *bool { return &v }

func TestBulkCreateRoutesReportsPartialFailures(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	if _, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/already-there", BaseURL: "https://a.example"}); err != nil {
		t.Fatalf("seed route: %v", err)
	}

	result, err := h.service.BulkCreateRoutes(ctx, project, []RouteInput{
		{Method: "GET", Path: "/ok-one", BaseURL: "https://a.example"},
		{Method: "NOPE", Path: "/bad-method", BaseURL: "https://a.example"},
		{Method: "POST", Path: "/ok-two", BaseURL: "https://a.example"},
		{Method: "GET", Path: "/already-there", BaseURL: "https://a.example"},
		{Method: "GET", Path: "/ok-one", BaseURL: "https://a.example"}, // duplicate within the batch
		{Method: "PUT", Path: "", BaseURL: "https://a.example"},
	})
	if err != nil {
		t.Fatalf("bulk create must not abort on bad rows: %v", err)
	}
	if len(result.Created) != 2 {
		t.Fatalf("expected 2 created routes, got %d", len(result.Created))
	}
	if len(result.Failed) != 4 {
		t.Fatalf("expected 4 reported failures, got %d: %+v", len(result.Failed), result.Failed)
	}
	// Every failure must identify its input row so the UI can highlight it.
	wantIndexes := map[int]string{
		1: domain.ErrInvalidRoute.Error(),
		3: domain.ErrDuplicateRoute.Error(),
		4: domain.ErrDuplicateRoute.Error(),
		5: "a route template is required",
	}
	for _, f := range result.Failed {
		want, ok := wantIndexes[f.Index]
		if !ok {
			t.Fatalf("unexpected failure at index %d: %+v", f.Index, f)
		}
		if f.Error != want {
			t.Fatalf("index %d: expected %q, got %q", f.Index, want, f.Error)
		}
		delete(wantIndexes, f.Index)
	}
	if len(wantIndexes) != 0 {
		t.Fatalf("missing expected failures: %+v", wantIndexes)
	}

	_, total, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 routes in the project (1 seeded + 2 created), got %d", total)
	}
}

func TestUpdateRoutePreservesUnsetFieldsAndClamps(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	route, err := h.service.CreateRoute(ctx, project, RouteInput{
		Method: "GET", Path: "/thing", BaseURL: "https://a.example", Summary: "original", Tags: []string{"core"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := h.service.UpdateRoute(ctx, route, RouteInput{TimeoutMS: 99999, Retries: 99, MonitorIntervalSecs: 1})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Summary != "original" {
		t.Fatalf("an empty summary must leave the stored value alone, got %q", updated.Summary)
	}
	if updated.TimeoutMS != 60000 || updated.Retries != 5 || updated.MonitorIntervalSecs != 10 {
		t.Fatalf("values not clamped: %+v", updated)
	}
}

func TestSetRouteEnabledDrivesDisabledState(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	route, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/toggle", BaseURL: "https://a.example"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err = h.service.SetRouteEnabled(ctx, route.ID, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	stored, _ := h.service.GetRoute(ctx, route.ID)
	if stored.Enabled {
		t.Fatal("expected the route to be disabled")
	}
	if stored.Status != domain.RouteStatusDisabled {
		t.Fatalf("a disabled route must report the disabled state, got %q", stored.Status)
	}

	if err = h.service.SetRouteEnabled(ctx, route.ID, true); err != nil {
		t.Fatalf("enable: %v", err)
	}
	stored, _ = h.service.GetRoute(ctx, route.ID)
	if stored.Status != domain.RouteStatusUnknown {
		t.Fatalf("a re-enabled, never-checked route must be unknown, got %q", stored.Status)
	}
}

func TestBulkDeleteRoutesIsScopedToTheProject(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	mine := seedProject(t, h)
	theirs, err := h.service.CreateProject(ctx, 2, CreateProjectInput{Name: "Other"})
	if err != nil {
		t.Fatalf("create other project: %v", err)
	}
	a, err := h.service.CreateRoute(ctx, mine, RouteInput{Method: "GET", Path: "/mine", BaseURL: "https://a.example"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	victim, err := h.service.CreateRoute(ctx, theirs, RouteInput{Method: "GET", Path: "/theirs", BaseURL: "https://a.example"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// A caller authorized for `mine` supplies a route ID from another project.
	deleted, err := h.service.BulkDeleteRoutes(ctx, mine.ID, []int64{a.ID, victim.ID})
	if err != nil {
		t.Fatalf("bulk delete: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected only the caller's own route to be deleted, got %d", deleted)
	}
	if survivor, _ := h.service.GetRoute(ctx, victim.ID); survivor == nil {
		t.Fatal("a route from another project must never be deleted through this path")
	}
}

func TestRedactHeaders(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want map[string]string
	}{
		{"empty", "", nil},
		{"malformed json yields nothing", "{not json", nil},
		{
			name: "sensitive values are masked, others preserved",
			in:   `{"Authorization":"Bearer secret","X-API-Key":"abc123","Cookie":"sid=1","X-Trace":"keepme"}`,
			want: map[string]string{"Authorization": "••••••••", "X-API-Key": "••••••••", "Cookie": "••••••••", "X-Trace": "keepme"},
		},
		{
			name: "matching is case-insensitive",
			in:   `{"authorization":"Bearer secret","x-auth-token":"t"}`,
			want: map[string]string{"authorization": "••••••••", "x-auth-token": "••••••••"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactHeaders(tc.in)
			if tc.want == nil {
				if got != "" {
					t.Fatalf("expected an empty result, got %q", got)
				}
				return
			}
			var parsed map[string]string
			if err := json.Unmarshal([]byte(got), &parsed); err != nil {
				t.Fatalf("result is not valid JSON: %v", err)
			}
			if len(parsed) != len(tc.want) {
				t.Fatalf("expected %d headers, got %d", len(tc.want), len(parsed))
			}
			for k, want := range tc.want {
				if parsed[k] != want {
					t.Fatalf("header %q: expected %q, got %q", k, want, parsed[k])
				}
			}
			if strings.Contains(got, "secret") || strings.Contains(got, "abc123") {
				t.Fatalf("a secret leaked through redaction: %s", got)
			}
		})
	}
}

// --- Acceptance criteria 3 & 4: check results update metrics; consecutive
// failures open exactly one incident and recovery resolves it. ---

func processChecks(t *testing.T, h *testHarness, routeID int64, outcomes ...string) models.APIRoute {
	t.Helper()
	ctx := context.Background()
	base := time.Now().UTC().Add(-time.Duration(len(outcomes)) * time.Minute)
	for i, outcome := range outcomes {
		route, err := h.service.GetRoute(ctx, routeID)
		if err != nil || route == nil {
			t.Fatalf("load route: %v", err)
		}
		statusCode, reason := 200, ""
		if outcome == "down" {
			statusCode, reason = 500, "status 500 outside expected range 200-399"
		}
		if err = h.service.ProcessRouteCheckResult(ctx, *route, outcome, statusCode, 42, reason, 1, base.Add(time.Duration(i)*time.Minute)); err != nil {
			t.Fatalf("process check %d: %v", i, err)
		}
	}
	final, err := h.service.GetRoute(ctx, routeID)
	if err != nil || final == nil {
		t.Fatalf("load final route: %v", err)
	}
	return *final
}

func TestProcessRouteCheckResultUpdatesMetrics(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	route, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/metrics", BaseURL: "https://a.example"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	final := processChecks(t, h, route.ID, "up")
	if final.Status != domain.RouteStatusHealthy {
		t.Fatalf("a successful check must make the route healthy, got %q", final.Status)
	}
	if final.LastStatusCode != 200 || final.LastLatencyMS != 42 {
		t.Fatalf("last check fields not recorded: %+v", final)
	}
	if final.LastCheckedAt == nil {
		t.Fatal("lastCheckedAt must be set")
	}
	if final.ConsecutiveSuccesses != 1 || final.ConsecutiveFailures != 0 {
		t.Fatalf("streak counters wrong: %+v", final)
	}
	expectedNext := final.LastCheckedAt.Add(time.Duration(route.MonitorIntervalSecs) * time.Second)
	if !final.NextCheckAt.Equal(expectedNext) {
		t.Fatalf("next check should be scheduled one interval later: got %s want %s", final.NextCheckAt, expectedNext)
	}

	checks, err := h.service.ListRouteChecks(ctx, route.ID, 10, 0)
	if err != nil {
		t.Fatalf("list checks: %v", err)
	}
	if len(checks) != 1 || checks[0].Status != "up" || checks[0].StatusCode != 200 {
		t.Fatalf("expected one persisted time-series row, got %+v", checks)
	}
}

func TestRouteIncidentAcknowledgementPreservesEvidenceAndAllowsResolution(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	route, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/incident-evidence", BaseURL: "https://a.example", FailureThreshold: 1})
	if err != nil {
		t.Fatal(err)
	}
	processChecks(t, h, route.ID, "down")
	open, err := h.service.ListRouteIncidents(ctx, project.ID, &route.ID, "open", 10, 0)
	if err != nil || len(open) != 1 {
		t.Fatalf("open incident: %v %+v", err, open)
	}
	if open[0].Source != "synthetic" || open[0].SourceKey != "route:"+strconv.FormatInt(route.ID, 10) || !strings.Contains(open[0].Evidence, `"statusCode":500`) {
		t.Fatalf("missing source/evidence: %+v", open[0])
	}
	if err = h.service.AcknowledgeRouteIncident(ctx, project.ID, open[0].ID, 99); err != nil {
		t.Fatalf("acknowledge: %v", err)
	}
	acknowledged, err := h.service.ListRouteIncidents(ctx, project.ID, &route.ID, "acknowledged", 10, 0)
	if err != nil || len(acknowledged) != 1 || acknowledged[0].AcknowledgedByID == nil || *acknowledged[0].AcknowledgedByID != 99 {
		t.Fatalf("acknowledged incident: %v %+v", err, acknowledged)
	}
	processChecks(t, h, route.ID, "up")
	resolved, err := h.service.ListRouteIncidents(ctx, project.ID, &route.ID, "resolved", 10, 0)
	if err != nil || len(resolved) != 1 || resolved[0].ResolvedAt == nil {
		t.Fatalf("resolved acknowledged incident: %v %+v", err, resolved)
	}
}

// TestRouteHealthStateProgression walks the full state machine a route moves
// through, which is the contract the dashboard renders.
func TestRouteHealthStateProgression(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	route, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/states", BaseURL: "https://a.example", FailureThreshold: 3, RecoverySuccesses: 1})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	steps := []struct {
		outcome    string
		wantStatus string
		wantOpen   int
	}{
		{"up", domain.RouteStatusHealthy, 0},
		{"down", domain.RouteStatusDegraded, 0}, // 1 failure, below threshold
		{"down", domain.RouteStatusDegraded, 0}, // 2 failures, still below
		{"down", domain.RouteStatusFailing, 1},  // 3 failures -> incident opens
		{"down", domain.RouteStatusFailing, 1},  // still failing, no second incident
		{"up", domain.RouteStatusHealthy, 0},    // 1 success -> incident resolves
	}
	for i, step := range steps {
		final := processChecks(t, h, route.ID, step.outcome)
		if final.Status != step.wantStatus {
			t.Fatalf("step %d (%s): expected status %q, got %q", i, step.outcome, step.wantStatus, final.Status)
		}
		if got := h.incidents.OpenCount(); got != step.wantOpen {
			t.Fatalf("step %d (%s): expected %d open incidents, got %d", i, step.outcome, step.wantOpen, got)
		}
	}

	if h.incidents.Openings != 1 {
		t.Fatalf("consecutive failures must open exactly ONE incident, got %d", h.incidents.Openings)
	}
	if h.incidents.Resolves != 1 {
		t.Fatalf("expected exactly one resolution, got %d", h.incidents.Resolves)
	}
	events := h.outbox.EventTypes()
	if len(events) != 2 || events[0] != "route_incident_opened" || events[1] != "route_incident_resolved" {
		t.Fatalf("expected one open and one resolve alert event, got %v", events)
	}
}

func TestIncidentThresholdsAreConfigurablePerRoute(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	t.Run("opens on the first failure when threshold is 1", func(t *testing.T) {
		route, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/hair-trigger", BaseURL: "https://a.example", FailureThreshold: 1, RecoverySuccesses: 1})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		final := processChecks(t, h, route.ID, "down")
		if final.Status != domain.RouteStatusFailing {
			t.Fatalf("expected failing after 1 failure, got %q", final.Status)
		}
		if h.incidents.OpenCount() != 1 {
			t.Fatalf("expected an incident to open immediately, got %d", h.incidents.OpenCount())
		}
	})

	t.Run("requires several successes to resolve when configured", func(t *testing.T) {
		route, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/slow-recovery", BaseURL: "https://a.example", FailureThreshold: 2, RecoverySuccesses: 3})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		before := h.incidents.Resolves
		processChecks(t, h, route.ID, "down", "down")
		open, err := h.service.ListRouteIncidents(ctx, project.ID, &route.ID, "open", 10, 0)
		if err != nil || len(open) != 1 {
			t.Fatalf("expected 1 open incident for this route, got %d (%v)", len(open), err)
		}

		processChecks(t, h, route.ID, "up")
		if h.incidents.Resolves != before {
			t.Fatal("one success must not resolve an incident that requires three")
		}
		processChecks(t, h, route.ID, "up")
		if h.incidents.Resolves != before {
			t.Fatal("two successes must not resolve an incident that requires three")
		}
		final := processChecks(t, h, route.ID, "up")
		if h.incidents.Resolves != before+1 {
			t.Fatal("the third success must resolve the incident")
		}
		if final.Status != domain.RouteStatusHealthy {
			t.Fatalf("expected healthy after recovery, got %q", final.Status)
		}
	})
}

func TestFailingRouteRecordsEveryCheckInHistory(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	route, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/history", BaseURL: "https://a.example"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	processChecks(t, h, route.ID, "up", "down", "down", "up")

	checks, err := h.service.ListRouteChecks(ctx, route.ID, 100, 0)
	if err != nil {
		t.Fatalf("list checks: %v", err)
	}
	if len(checks) != 4 {
		t.Fatalf("expected 4 time-series rows, got %d", len(checks))
	}
	// Newest first, matching the adapter's ORDER BY checked_at DESC.
	if checks[0].Status != "up" || checks[len(checks)-1].Status != "up" {
		t.Fatalf("unexpected ordering: %+v", checks)
	}
	failures := 0
	for _, c := range checks {
		if c.Status == "down" {
			failures++
			if c.StatusCode != 500 || c.FailureReason == "" {
				t.Fatalf("a failed check must record its code and reason: %+v", c)
			}
		}
	}
	if failures != 2 {
		t.Fatalf("expected 2 recorded failures, got %d", failures)
	}
}

func TestListRoutesSearchFilterSortPaginate(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	specs := []struct {
		method, path string
		tags         []string
		deprecated   bool
	}{
		{"GET", "/pets", []string{"pets"}, false},
		{"POST", "/pets", []string{"pets"}, false},
		{"GET", "/pets/{id}", []string{"pets"}, true},
		{"GET", "/orders", []string{"orders"}, false},
		{"DELETE", "/orders/{id}", []string{"orders"}, false},
	}
	for _, s := range specs {
		if _, err := h.service.CreateRoute(ctx, project, RouteInput{
			Method: s.method, Path: s.path, BaseURL: "https://a.example", Tags: s.tags, Deprecated: s.deprecated,
		}); err != nil {
			t.Fatalf("create %s %s: %v", s.method, s.path, err)
		}
	}

	t.Run("search", func(t *testing.T) {
		items, total, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Search: "order"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if total != 2 || len(items) != 2 {
			t.Fatalf("expected 2 order routes, got %d", total)
		}
	})

	t.Run("method filter", func(t *testing.T) {
		_, total, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Method: "GET"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if total != 3 {
			t.Fatalf("expected 3 GET routes, got %d", total)
		}
	})

	t.Run("tag filter", func(t *testing.T) {
		_, total, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Tag: "orders"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if total != 2 {
			t.Fatalf("expected 2 orders-tagged routes, got %d", total)
		}
	})

	t.Run("deprecated filter", func(t *testing.T) {
		deprecated := true
		_, total, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Deprecated: &deprecated})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if total != 1 {
			t.Fatalf("expected 1 deprecated route, got %d", total)
		}
	})

	t.Run("status filter", func(t *testing.T) {
		_, total, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Status: domain.RouteStatusUnknown})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if total != 5 {
			t.Fatalf("expected all 5 routes to be unknown, got %d", total)
		}
	})

	t.Run("pagination reports the unpaged total", func(t *testing.T) {
		page1, total, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Limit: 2, Offset: 0, SortBy: "path"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if total != 5 {
			t.Fatalf("expected total=5 regardless of the page size, got %d", total)
		}
		if len(page1) != 2 {
			t.Fatalf("expected a page of 2, got %d", len(page1))
		}
		page3, _, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Limit: 2, Offset: 4, SortBy: "path"})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(page3) != 1 {
			t.Fatalf("expected a final page of 1, got %d", len(page3))
		}
		if page1[0].Path == page3[0].Path {
			t.Fatal("pages must not overlap")
		}
	})

	t.Run("routes from other projects are never returned", func(t *testing.T) {
		other, err := h.service.CreateProject(ctx, 2, CreateProjectInput{Name: "Other"})
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if _, err = h.service.CreateRoute(ctx, other, RouteInput{Method: "GET", Path: "/secret", BaseURL: "https://a.example"}); err != nil {
			t.Fatalf("create: %v", err)
		}
		items, _, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		for _, item := range items {
			if item.Path == "/secret" {
				t.Fatal("cross-project leak in ListRoutes")
			}
		}
	})
}

// TestListRoutesScalesToThousands exercises the filter/sort/paginate path at
// the scale the requirements call for.
func TestListRoutesScalesToThousands(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	const total = 2000
	inputs := make([]RouteInput, 0, total)
	for i := 0; i < total; i++ {
		inputs = append(inputs, RouteInput{
			Method:  []string{"GET", "POST", "PUT", "DELETE"}[i%4],
			Path:    fmt.Sprintf("/resource-%04d/items", i),
			BaseURL: "https://a.example",
		})
	}
	result, err := h.service.BulkCreateRoutes(ctx, project, inputs)
	if err != nil {
		t.Fatalf("bulk create: %v", err)
	}
	if len(result.Failed) != 0 {
		t.Fatalf("expected no failures, got %d", len(result.Failed))
	}

	_, got, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if got != total {
		t.Fatalf("expected %d routes, got %d", total, got)
	}

	page, _, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Search: "resource-1234", Limit: 50})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(page) != 1 || page[0].Path != "/resource-1234/items" {
		t.Fatalf("search at scale failed: %+v", page)
	}
}
