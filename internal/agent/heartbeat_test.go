package agent

import (
	"context"
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
