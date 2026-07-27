package domain

import "testing"

func TestComputeRouteStatus(t *testing.T) {
	cases := []struct {
		name string
		in   RouteHealthInput
		want string
	}{
		{"disabled wins over everything", RouteHealthInput{Enabled: false, Checked: true, LastStatus: "up"}, RouteStatusDisabled},
		{"never checked", RouteHealthInput{Enabled: true, Checked: false}, RouteStatusUnknown},
		{"healthy on success streak", RouteHealthInput{Enabled: true, Checked: true, LastStatus: "up", ConsecutiveFailures: 0}, RouteStatusHealthy},
		{"degraded below threshold", RouteHealthInput{Enabled: true, Checked: true, LastStatus: "down", ConsecutiveFailures: 1, FailureThreshold: 3}, RouteStatusDegraded},
		{"failing at threshold", RouteHealthInput{Enabled: true, Checked: true, LastStatus: "down", ConsecutiveFailures: 3, FailureThreshold: 3}, RouteStatusFailing},
		{"failing above threshold", RouteHealthInput{Enabled: true, Checked: true, LastStatus: "down", ConsecutiveFailures: 9, FailureThreshold: 3}, RouteStatusFailing},
		{"defaults threshold when unset", RouteHealthInput{Enabled: true, Checked: true, LastStatus: "down", ConsecutiveFailures: DefaultFailureThreshold}, RouteStatusFailing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComputeRouteStatus(tc.in); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestRouteIncidentPolicyOpensAfterConsecutiveFailures(t *testing.T) {
	// Below threshold: no transition.
	tr := RouteIncidentPolicy(false, 2, 3, 0, 1)
	if tr.ShouldOpen || tr.ShouldResolve {
		t.Fatalf("expected no transition below threshold, got %+v", tr)
	}
	// At threshold: opens exactly once.
	tr = RouteIncidentPolicy(false, 3, 3, 0, 1)
	if !tr.ShouldOpen {
		t.Fatalf("expected incident to open at failure threshold")
	}
	// Already open + still failing: no repeated open.
	tr = RouteIncidentPolicy(true, 5, 3, 0, 1)
	if tr.ShouldOpen || tr.ShouldResolve {
		t.Fatalf("expected no-op while incident already open and still failing, got %+v", tr)
	}
}

func TestRouteIncidentPolicyResolvesAfterRecoverySuccesses(t *testing.T) {
	// One success, but recovery requires two: no resolve yet.
	tr := RouteIncidentPolicy(true, 0, 3, 1, 2)
	if tr.ShouldResolve {
		t.Fatalf("expected incident to remain open before recovery threshold met")
	}
	tr = RouteIncidentPolicy(true, 0, 3, 2, 2)
	if !tr.ShouldResolve {
		t.Fatalf("expected incident to resolve once recovery threshold met")
	}
	// Not open: resolving is a no-op regardless of successes.
	tr = RouteIncidentPolicy(false, 0, 3, 5, 2)
	if tr.ShouldResolve {
		t.Fatalf("expected no resolve transition when no incident is open")
	}
}

func TestNormalizeMethodAndPath(t *testing.T) {
	m, err := NormalizeMethod("get")
	if err != nil || m != "GET" {
		t.Fatalf("expected GET, got %q err=%v", m, err)
	}
	if _, err = NormalizeMethod("BREW"); err == nil {
		t.Fatal("expected error for unsupported method")
	}
	p, err := NormalizePath("pets/{id}/")
	if err != nil || p != "/pets/{id}" {
		t.Fatalf("expected normalized path, got %q err=%v", p, err)
	}
	if _, err = NormalizePath("   "); err == nil {
		t.Fatal("expected error for empty path")
	}
}
