package worker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"argus/internal/models"
)

func testEvaluator(server *httptest.Server) *RouteEvaluator {
	client := server.Client()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRouteRedirects {
			return errors.New("too many redirects")
		}
		return nil
	}
	return &RouteEvaluator{
		validate: func(context.Context, string) error { return nil },
		client:   client,
	}
}

func routeFor(server *httptest.Server) models.APIRoute {
	return models.APIRoute{
		Method:              http.MethodGet,
		Path:                "/health",
		BaseURL:             server.URL,
		TimeoutMS:           1000,
		ExpectedStatusRange: "200-399",
	}
}

func TestValidateTargetRejectsUnsafeTargets(t *testing.T) {
	t.Parallel()
	tests := []string{
		"http://127.0.0.1/admin",
		"http://[::1]/admin",
		"http://169.254.169.254/latest/meta-data",
		"http://metadata.google.internal/computeMetadata/v1",
		"file:///etc/passwd",
		"http://user:password@example.com",
	}
	for _, target := range tests {
		target := target
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			if err := validateTargetContext(context.Background(), target); err == nil {
				t.Fatalf("expected %q to be rejected", target)
			}
		})
	}
}

func TestRouteEvaluatorExpectedStatusAndMethod(t *testing.T) {
	t.Parallel()
	var method string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusTeapot)
	}))
	defer server.Close()
	route := routeFor(server)
	route.Method = http.MethodHead
	route.ExpectedStatusRange = "200-299,418"

	result := testEvaluator(server).Evaluate(context.Background(), route, 2*time.Second)
	if result.Status != "up" || result.StatusCode != http.StatusTeapot {
		t.Fatalf("unexpected result: %+v", result)
	}
	if method != http.MethodHead {
		t.Fatalf("expected HEAD, got %s", method)
	}
}

func TestRouteEvaluatorPreservesServerBasePath(t *testing.T) {
	t.Parallel()
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	route := routeFor(server)
	route.BaseURL = server.URL + "/api/v1"
	route.Path = "/health"
	result := testEvaluator(server).Evaluate(context.Background(), route, time.Second)
	if result.Status != "up" || path != "/api/v1/health" {
		t.Fatalf("unexpected result=%+v path=%q", result, path)
	}
}

func TestRouteEvaluatorRetriesAndCountsFinalAttempt(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	route := routeFor(server)
	route.Retries = 2

	result := testEvaluator(server).Evaluate(context.Background(), route, 2*time.Second)
	if result.Status != "up" || result.Attempts != 3 || calls.Load() != 3 {
		t.Fatalf("unexpected retry result: %+v, calls=%d", result, calls.Load())
	}
}

func TestRouteEvaluatorTimeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	route := routeFor(server)
	route.TimeoutMS = 25

	result := testEvaluator(server).Evaluate(context.Background(), route, time.Second)
	if result.Status != "down" || result.Attempts != 1 {
		t.Fatalf("unexpected timeout result: %+v", result)
	}
}

func TestRouteEvaluatorSubstitutesPathParameters(t *testing.T) {
	t.Parallel()
	var path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	route := routeFor(server)
	route.Path = "/users/{userId}/posts/{postId}"
	route.Parameters = `[{"name":"userId","in":"path","example":"alice"},{"name":"postId","in":"path","default":"42"}]`

	result := testEvaluator(server).Evaluate(context.Background(), route, time.Second)
	if result.Status != "up" || path != "/users/alice/posts/42" {
		t.Fatalf("unexpected result=%+v path=%q", result, path)
	}
}

func TestRedirectIsRevalidated(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, "http://169.254.169.254/latest/meta-data", http.StatusFound)
	}))
	defer server.Close()

	evaluator := testEvaluator(server)
	evaluator.client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
		return validateTargetContext(req.Context(), req.URL.String())
	}
	route := routeFor(server)
	result := evaluator.Evaluate(context.Background(), route, time.Second)
	if result.Status != "down" {
		t.Fatalf("redirect to metadata endpoint must fail: %+v", result)
	}
}
