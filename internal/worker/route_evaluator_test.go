package worker

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"argus/internal/models"
)

// testEvaluator builds an evaluator that is allowed to reach loopback so it
// can be pointed at httptest servers. Cloud metadata ranges stay blocked even
// in this mode, which the SSRF tests below rely on.
func testEvaluator(t *testing.T) *RouteEvaluator {
	t.Helper()
	return NewRouteEvaluator(EvaluatorConfig{AllowPrivateTargets: true, MaxTimeout: 5 * time.Second})
}

// strictEvaluator uses the production default policy: private, loopback and
// link-local targets are refused.
func strictEvaluator(t *testing.T) *RouteEvaluator {
	t.Helper()
	return NewRouteEvaluator(EvaluatorConfig{MaxTimeout: 5 * time.Second})
}

func routeFor(base, method, path string) models.APIRoute {
	return models.APIRoute{
		ID: 1, ProjectID: 1, Method: method, Path: path, BaseURL: base,
		Enabled: true, MonitorIntervalSecs: 60, TimeoutMS: 2000,
		ExpectedStatusRange: "200-399", FailureThreshold: 3, RecoverySuccesses: 1,
	}
}

func TestEvaluateRouteSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	outcome := testEvaluator(t).EvaluateRoute(context.Background(), routeFor(srv.URL, "POST", "/widgets"))
	if outcome.Status != "up" {
		t.Fatalf("expected up, got %s (%s)", outcome.Status, outcome.FailureReason)
	}
	if outcome.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", outcome.StatusCode)
	}
	if outcome.Attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", outcome.Attempts)
	}
}

func TestEvaluateRouteExpectedStatusRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	e := testEvaluator(t)

	t.Run("outside range fails", func(t *testing.T) {
		outcome := e.EvaluateRoute(context.Background(), routeFor(srv.URL, "GET", "/missing"))
		if outcome.Status != "down" {
			t.Fatalf("expected down, got %s", outcome.Status)
		}
		if !strings.Contains(outcome.FailureReason, "outside expected range") {
			t.Fatalf("unexpected reason: %s", outcome.FailureReason)
		}
	})

	t.Run("explicitly expected 404 succeeds", func(t *testing.T) {
		route := routeFor(srv.URL, "GET", "/missing")
		route.ExpectedStatusRange = "200-204,404"
		outcome := e.EvaluateRoute(context.Background(), route)
		if outcome.Status != "up" {
			t.Fatalf("expected up, got %s (%s)", outcome.Status, outcome.FailureReason)
		}
	})

	t.Run("malformed range fails closed", func(t *testing.T) {
		route := routeFor(srv.URL, "GET", "/missing")
		route.ExpectedStatusRange = "abc"
		outcome := e.EvaluateRoute(context.Background(), route)
		if outcome.Status != "down" || !strings.Contains(outcome.FailureReason, "invalid expected status range") {
			t.Fatalf("expected invalid-range failure, got %s / %s", outcome.Status, outcome.FailureReason)
		}
	})
}

func TestEvaluateRouteRetriesAndCountsAttempts(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&hits, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	route := routeFor(srv.URL, "GET", "/flaky")
	route.Retries = 3
	outcome := testEvaluator(t).EvaluateRoute(context.Background(), route)
	if outcome.Status != "up" {
		t.Fatalf("expected recovery on retry, got %s (%s)", outcome.Status, outcome.FailureReason)
	}
	if outcome.Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", outcome.Attempts)
	}
}

func TestEvaluateRouteExhaustsRetries(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	route := routeFor(srv.URL, "GET", "/always-bad")
	route.Retries = 2
	outcome := testEvaluator(t).EvaluateRoute(context.Background(), route)
	if outcome.Status != "down" {
		t.Fatalf("expected down, got %s", outcome.Status)
	}
	if outcome.Attempts != 3 {
		t.Fatalf("expected 3 attempts (1 + 2 retries), got %d", outcome.Attempts)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("expected 3 requests, server saw %d", got)
	}
}

func TestEvaluateRouteTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer func() { close(release); srv.Close() }()

	route := routeFor(srv.URL, "GET", "/slow")
	route.TimeoutMS = 250
	start := time.Now()
	outcome := testEvaluator(t).EvaluateRoute(context.Background(), route)
	elapsed := time.Since(start)

	if outcome.Status != "down" {
		t.Fatalf("expected down, got %s", outcome.Status)
	}
	if !strings.Contains(outcome.FailureReason, "timed out") {
		t.Fatalf("expected timeout reason, got %q", outcome.FailureReason)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("timeout was not enforced, took %s", elapsed)
	}
}

func TestEvaluateRouteAppliesCustomHeaders(t *testing.T) {
	var gotAuth, gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	route := routeFor(srv.URL, "GET", "/secured")
	route.Headers = `{"Authorization":"Bearer s3cret","Content-Length":"999"}`
	if outcome := testEvaluator(t).EvaluateRoute(context.Background(), route); outcome.Status != "up" {
		t.Fatalf("expected up, got %s (%s)", outcome.Status, outcome.FailureReason)
	}
	if gotAuth != "Bearer s3cret" {
		t.Fatalf("expected configured Authorization header to be sent, got %q", gotAuth)
	}
	if gotUA == "" {
		t.Fatal("expected a User-Agent to be set")
	}
}

func TestEvaluateRouteSubstitutesPathParameters(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	route := routeFor(srv.URL, "GET", "/pets/{petId}/toys/{toyName}")
	route.Parameters = `[{"name":"petId","in":"path","type":"integer","example":"42"},
	                     {"name":"toyName","in":"path","type":"string"}]`
	if outcome := testEvaluator(t).EvaluateRoute(context.Background(), route); outcome.Status != "up" {
		t.Fatalf("expected up, got %s (%s)", outcome.Status, outcome.FailureReason)
	}
	if gotPath != "/pets/42/toys/sample" {
		t.Fatalf("unexpected substituted path %q", gotPath)
	}
}

func TestEvaluateRouteFollowsSafeRedirect(t *testing.T) {
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer final.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/final", http.StatusFound)
	}))
	defer redirector.Close()

	outcome := testEvaluator(t).EvaluateRoute(context.Background(), routeFor(redirector.URL, "GET", "/start"))
	if outcome.Status != "up" {
		t.Fatalf("expected up after redirect, got %s (%s)", outcome.Status, outcome.FailureReason)
	}
}

func TestEvaluateRouteRejectsRedirectToBlockedTarget(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer redirector.Close()

	// Even with the private-network policy relaxed, redirecting into the
	// cloud metadata range must be refused.
	outcome := testEvaluator(t).EvaluateRoute(context.Background(), routeFor(redirector.URL, "GET", "/start"))
	if outcome.Status != "down" {
		t.Fatalf("expected redirect to metadata endpoint to fail, got %s", outcome.Status)
	}
	if !strings.Contains(outcome.FailureReason, "redirect rejected") && !strings.Contains(outcome.FailureReason, ErrBlockedTarget.Error()) {
		t.Fatalf("expected blocked-redirect reason, got %q", outcome.FailureReason)
	}
}

func TestEvaluateRouteCapsRedirects(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/loop", http.StatusFound)
	}))
	defer srv.Close()

	e := NewRouteEvaluator(EvaluatorConfig{AllowPrivateTargets: true, MaxRedirects: 3, MaxTimeout: 5 * time.Second})
	outcome := e.EvaluateRoute(context.Background(), routeFor(srv.URL, "GET", "/loop"))
	if outcome.Status != "down" {
		t.Fatalf("expected redirect loop to fail, got %s", outcome.Status)
	}
	if !strings.Contains(outcome.FailureReason, "stopped after 3 redirects") {
		t.Fatalf("expected redirect cap reason, got %q", outcome.FailureReason)
	}
}

func TestEvaluateRouteStripsSecretsOnCrossOriginRedirect(t *testing.T) {
	var leaked string
	var seen int32
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&seen, 1)
		leaked = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer final.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, final.URL+"/final", http.StatusFound)
	}))
	defer redirector.Close()

	route := routeFor(redirector.URL, "GET", "/start")
	route.Headers = `{"Authorization":"Bearer s3cret"}`
	if outcome := testEvaluator(t).EvaluateRoute(context.Background(), route); outcome.Status != "up" {
		t.Fatalf("expected up, got %s (%s)", outcome.Status, outcome.FailureReason)
	}
	if atomic.LoadInt32(&seen) != 1 {
		t.Fatal("expected the redirect target to be reached")
	}
	if leaked != "" {
		t.Fatalf("Authorization header leaked across origins: %q", leaked)
	}
}

func TestEvaluateRouteBlocksUnsafeTargets(t *testing.T) {
	e := strictEvaluator(t)
	cases := []struct {
		name string
		base string
		path string
	}{
		{"loopback ipv4", "http://127.0.0.1:9", "/x"},
		{"loopback ipv6", "http://[::1]:9", "/x"},
		{"localhost name", "http://localhost:9", "/x"},
		{"rfc1918 10/8", "http://10.0.0.5", "/x"},
		{"rfc1918 192.168/16", "http://192.168.1.1", "/x"},
		{"rfc1918 172.16/12", "http://172.16.4.4", "/x"},
		{"aws metadata", "http://169.254.169.254", "/latest/meta-data/"},
		{"gcp metadata", "http://metadata.google.internal", "/computeMetadata/v1/"},
		{"link local", "http://169.254.1.1", "/x"},
		{"cgnat", "http://100.64.0.1", "/x"},
		{"unspecified", "http://0.0.0.0", "/x"},
		{"file scheme", "file:///etc/passwd", "/x"},
		{"gopher scheme", "gopher://example.com", "/x"},
		{"embedded credentials", "http://user:pass@example.com", "/x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome := e.EvaluateRoute(context.Background(), routeFor(tc.base, "GET", tc.path))
			if outcome.Status != "down" {
				t.Fatalf("expected %s to be blocked, got %s", tc.base, outcome.Status)
			}
			if outcome.Attempts != 1 {
				t.Fatalf("blocked targets must not be retried, got %d attempts", outcome.Attempts)
			}
			if outcome.StatusCode != 0 {
				t.Fatalf("expected no HTTP exchange, got status %d", outcome.StatusCode)
			}
		})
	}
}

// TestEvaluateRouteBlocksDNSRebinding proves the policy is applied to the
// resolved IP at connect time, not just to the hostname: a public-looking
// hostname that resolves to loopback is still refused.
func TestEvaluateRouteBlocksDNSRebinding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// localtest.me and its subdomains are public DNS names that resolve to
	// 127.0.0.1. If DNS resolution fails in the sandbox the test is skipped.
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")
	target := fmt.Sprintf("http://rebind.localtest.me:%s", port)
	outcome := strictEvaluator(t).EvaluateRoute(context.Background(), routeFor(target, "GET", "/x"))
	if outcome.Status == "up" {
		t.Fatal("a hostname resolving to loopback must be refused")
	}
	if strings.Contains(outcome.FailureReason, "no such host") || strings.Contains(outcome.FailureReason, "server misbehaving") {
		t.Skipf("DNS unavailable in this environment: %s", outcome.FailureReason)
	}
	if !strings.Contains(outcome.FailureReason, ErrBlockedTarget.Error()) {
		t.Fatalf("expected a blocked-target reason, got %q", outcome.FailureReason)
	}
}

func TestEvaluateRouteRequiresBaseURL(t *testing.T) {
	route := routeFor("", "GET", "/x")
	outcome := strictEvaluator(t).EvaluateRoute(context.Background(), route)
	if outcome.Status != "down" || !strings.Contains(outcome.FailureReason, "no base URL") {
		t.Fatalf("expected missing base URL failure, got %s / %s", outcome.Status, outcome.FailureReason)
	}
}

func TestValidateAddrAllowsPublicAddresses(t *testing.T) {
	e := strictEvaluator(t)
	for _, ip := range []string{"93.184.216.34", "8.8.8.8", "2606:2800:220:1:248:1893:25c8:1946"} {
		if err := e.validateAddr(netip.MustParseAddr(ip)); err != nil {
			t.Fatalf("expected %s to be allowed, got %v", ip, err)
		}
	}
}

func TestParseStatusRange(t *testing.T) {
	cases := []struct {
		expr    string
		code    int
		want    bool
		wantErr bool
	}{
		{"", 250, true, false},
		{"", 404, false, false},
		{"200-399", 399, true, false},
		{"200-399", 400, false, false},
		{"200,201,204", 204, true, false},
		{"200,201,204", 202, false, false},
		{"200-204,301", 301, true, false},
		{"99-200", 150, false, true},
		{"200-600", 300, false, true},
		{"399-200", 300, false, true},
		{"nope", 200, false, true},
	}
	for _, tc := range cases {
		got, err := ParseStatusRange(tc.expr)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseStatusRange(%q): expected error", tc.expr)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseStatusRange(%q): unexpected error %v", tc.expr, err)
			continue
		}
		if got.contains(tc.code) != tc.want {
			t.Errorf("ParseStatusRange(%q).contains(%d) = %v, want %v", tc.expr, tc.code, !tc.want, tc.want)
		}
	}
}

func TestBuildRouteURL(t *testing.T) {
	cases := []struct {
		name   string
		route  models.APIRoute
		want   string
		errStr string
	}{
		{
			name:  "plain path",
			route: models.APIRoute{BaseURL: "https://api.example.com/v1/", Path: "/pets"},
			want:  "https://api.example.com/v1/pets",
		},
		{
			name:  "brace parameter falls back to synthetic id",
			route: models.APIRoute{BaseURL: "https://api.example.com", Path: "/pets/{petId}"},
			want:  "https://api.example.com/pets/1",
		},
		{
			name:  "colon parameter",
			route: models.APIRoute{BaseURL: "https://api.example.com", Path: "/pets/:name/feed"},
			want:  "https://api.example.com/pets/sample/feed",
		},
		{
			name: "example value is escaped so it cannot inject path segments",
			route: models.APIRoute{
				BaseURL:    "https://api.example.com",
				Path:       "/pets/{petId}",
				Parameters: `[{"name":"petId","in":"path","example":"../../admin?x=1"}]`,
			},
			want: "https://api.example.com/pets/..%2F..%2Fadmin%3Fx=1",
		},
		{
			name:   "missing base URL",
			route:  models.APIRoute{Path: "/pets"},
			errStr: "no base URL",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := BuildRouteURL(tc.route)
			if tc.errStr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errStr) {
					t.Fatalf("expected error containing %q, got %v", tc.errStr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("BuildRouteURL = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDescribeTransportErrorUnwrapsBlockedTarget(t *testing.T) {
	wrapped := fmt.Errorf(`Get "http://10.0.0.1": dial tcp: %w: 10.0.0.1 is a private or loopback address`, ErrBlockedTarget)
	got := describeTransportError(wrapped, time.Second)
	if !strings.HasPrefix(got, ErrBlockedTarget.Error()) {
		t.Fatalf("expected reason to start with the blocked-target sentinel, got %q", got)
	}
	if !errors.Is(wrapped, ErrBlockedTarget) {
		t.Fatal("sentinel should remain matchable")
	}
}
