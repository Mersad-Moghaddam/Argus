package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateTargetBlocksMetadata(t *testing.T) {
	if err := validateTarget("http://169.254.169.254/latest"); err == nil {
		t.Fatal("expected metadata endpoint to be blocked")
	}
}

func TestLegacyWebsiteHTTPCheckUsesStrictEgressPolicy(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer server.Close()
	status, _, _, reason := (&Processor{}).checkHTTP(context.Background(), server.URL)
	if status != "down" || reason == "" {
		t.Fatalf("loopback legacy target: status=%q reason=%q", status, reason)
	}
	if called {
		t.Fatal("strict legacy evaluator must not dial loopback targets")
	}
}
