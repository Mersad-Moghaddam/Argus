package application

import (
	"context"
	"testing"
	"time"

	"argus/internal/models"
)

func TestListMetricsTimeseriesBucketsChecks(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	route, err := h.service.CreateRoute(ctx, project, RouteInput{Method: "GET", Path: "/series", BaseURL: "https://a.example"})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	// Two checks in one 30-minute bucket (one up, one down) and one in another.
	now := time.Now().UTC()
	seed := []struct {
		at        time.Time
		status    string
		latencyMS int
	}{
		{now.Add(-90 * time.Minute), "up", 100},
		{now.Add(-88 * time.Minute), "down", 300},
		{now.Add(-10 * time.Minute), "up", 50},
	}
	for _, s := range seed {
		if err = h.routes.RecordRouteCheck(ctx, models.RouteCheck{
			RouteID: route.ID, ProjectID: project.ID, Status: s.status,
			LatencyMS: s.latencyMS, CheckedAt: s.at,
		}); err != nil {
			t.Fatalf("record check: %v", err)
		}
	}

	series, err := h.service.ListMetricsTimeseries(ctx, project.ID, nil, "24h")
	if err != nil {
		t.Fatalf("timeseries: %v", err)
	}
	if series.Range != "24h" || series.BucketSeconds != 1800 {
		t.Fatalf("unexpected window: %+v", series.TimeseriesWindow)
	}
	if len(series.Points) != 2 {
		t.Fatalf("expected 2 buckets, got %d (%+v)", len(series.Points), series.Points)
	}

	first := series.Points[0]
	if first.Checks != 2 || first.Failures != 1 {
		t.Fatalf("expected the first bucket to hold 2 checks / 1 failure, got %+v", first)
	}
	if first.UptimePct != 50 {
		t.Fatalf("expected 50%% uptime in the first bucket, got %v", first.UptimePct)
	}
	if first.AvgLatencyMS != 200 || first.MaxLatencyMS != 300 {
		t.Fatalf("unexpected latency aggregation: %+v", first)
	}

	last := series.Points[1]
	if last.Checks != 1 || last.Failures != 0 || last.UptimePct != 100 {
		t.Fatalf("expected a clean second bucket, got %+v", last)
	}
	if !series.Points[0].BucketStart.Before(series.Points[1].BucketStart) {
		t.Fatal("buckets must be ordered oldest first")
	}
}

func TestListMetricsTimeseriesRangeHandling(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	cases := []struct {
		rangeKey    string
		wantRange   string
		wantBuckets int
	}{
		{"1h", "1h", 120},
		{"6h", "6h", 600},
		{"24h", "24h", 1800},
		{"7d", "7d", 10800},
		{"30d", "30d", 86400},
		{"  24H  ", "24h", 1800},                        // trimmed and lower-cased
		{"", DefaultTimeseriesRange, 1800},              // omitted falls back
		{"all-of-time", DefaultTimeseriesRange, 1800},   // unknown falls back
		{"'; DROP TABLE", DefaultTimeseriesRange, 1800}, // never interpolated
	}
	for _, tc := range cases {
		t.Run(tc.rangeKey, func(t *testing.T) {
			series, err := h.service.ListMetricsTimeseries(ctx, project.ID, nil, tc.rangeKey)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if series.Range != tc.wantRange || series.BucketSeconds != tc.wantBuckets {
				t.Fatalf("expected %s/%ds, got %s/%ds", tc.wantRange, tc.wantBuckets, series.Range, series.BucketSeconds)
			}
			// Every named range must produce a small, renderable bucket count.
			span := timeseriesSpans[series.Range]
			if buckets := int(span.Seconds()) / series.BucketSeconds; buckets > 100 {
				t.Fatalf("range %s would produce %d buckets, too many to chart", series.Range, buckets)
			}
			if series.Points == nil {
				t.Fatal("points must be an empty slice, never null, so the chart can render an empty state")
			}
		})
	}
}

func TestListMetricsTimeseriesIsScopedToProjectAndRoute(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	mine := seedProject(t, h)
	theirs, err := h.service.CreateProject(ctx, 2, CreateProjectInput{Name: "Other"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	routeA, err := h.service.CreateRoute(ctx, mine, RouteInput{Method: "GET", Path: "/a", BaseURL: "https://a.example"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	routeB, err := h.service.CreateRoute(ctx, mine, RouteInput{Method: "GET", Path: "/b", BaseURL: "https://a.example"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	routeOther, err := h.service.CreateRoute(ctx, theirs, RouteInput{Method: "GET", Path: "/x", BaseURL: "https://a.example"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	at := time.Now().UTC().Add(-5 * time.Minute)
	for _, seed := range []struct {
		route   models.APIRoute
		project models.Project
	}{{routeA, mine}, {routeB, mine}, {routeOther, theirs}} {
		if err = h.routes.RecordRouteCheck(ctx, models.RouteCheck{
			RouteID: seed.route.ID, ProjectID: seed.project.ID, Status: "up", LatencyMS: 10, CheckedAt: at,
		}); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	all, err := h.service.ListMetricsTimeseries(ctx, mine.ID, nil, "1h")
	if err != nil {
		t.Fatalf("timeseries: %v", err)
	}
	total := 0
	for _, p := range all.Points {
		total += p.Checks
	}
	if total != 2 {
		t.Fatalf("expected only this project's 2 checks, got %d", total)
	}

	single, err := h.service.ListMetricsTimeseries(ctx, mine.ID, &routeA.ID, "1h")
	if err != nil {
		t.Fatalf("timeseries: %v", err)
	}
	total = 0
	for _, p := range single.Points {
		total += p.Checks
	}
	if total != 1 {
		t.Fatalf("expected 1 check for the single route, got %d", total)
	}
}
