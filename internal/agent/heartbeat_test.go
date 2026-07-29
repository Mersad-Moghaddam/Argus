package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testToken = "argus_agent_abcdefghijklmnopqrstuvwxyz"

func TestClientSendsOutboundBoundHeartbeat(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != heartbeatPath || r.Method != http.MethodPost {
			t.Fatalf("request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
			t.Fatalf("authorization %q", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	client, err := NewClient(Config{ControlURL: server.URL, Token: testToken, Version: "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Heartbeat(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestClientRejectsInsecureNonLoopbackControlURL(t *testing.T) {
	if _, err := NewClient(Config{ControlURL: "http://argus.example.test", Token: testToken}); err == nil {
		t.Fatal("expected insecure control URL to be rejected")
	}
}

func TestClientReportsBoundedResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != resultPath || r.Method != http.MethodPost {
			t.Fatalf("request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
			t.Fatalf("authorization %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "agent-result-key-0001" {
			t.Fatalf("idempotency key %q", got)
		}
		var body struct {
			Outcome string `json:"outcome"`
			Summary string `json:"summary"`
			Version string `json:"version"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Outcome != "failure" || body.Summary != "bounded failure" || body.Version != "1.2.3" {
			t.Fatalf("body: %+v", body)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	client, err := NewClient(Config{ControlURL: server.URL, Token: testToken, Version: "1.2.3"})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.ReportResult(context.Background(), "agent-result-key-0001", "failure", "bounded failure"); err != nil {
		t.Fatal(err)
	}
	if err = client.ReportResult(context.Background(), "too-short", "failure", "bounded failure"); err == nil {
		t.Fatal("expected invalid result rejection")
	}
}
