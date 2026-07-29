package notifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"argus/internal/models"
)

func TestNotifierDoesNotFollowWebhookRedirects(t *testing.T) {
	followed := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/next" {
			followed = true
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Redirect(w, r, "/next", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	n := NewHTTPNotifier()
	client := server.Client()
	client.CheckRedirect = n.client.CheckRedirect
	n.client = client
	if err := n.Notify(context.Background(), []models.AlertChannel{{ChannelType: "webhook", Target: server.URL, Enabled: true}}, []byte(`{"event":"incident"}`)); err == nil {
		t.Fatal("expected redirecting notification to be reported as a failure")
	}
	if followed {
		t.Fatal("notification payload followed a redirect")
	}
}

func TestNotifierSkipsDisabledChannelsAndReportsDeliveryFailure(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	n := NewHTTPNotifier()
	if err := n.Notify(context.Background(), []models.AlertChannel{
		{ChannelType: "webhook", Target: server.URL, Enabled: false},
	}, []byte(`{"event":"incident"}`)); err != nil {
		t.Fatalf("disabled channel error: %v", err)
	}
	if called {
		t.Fatal("disabled channel received a notification")
	}
	if err := n.Notify(context.Background(), []models.AlertChannel{
		{ChannelType: "webhook", Target: server.URL, Enabled: true},
	}, []byte(`{"event":"incident"}`)); err == nil {
		t.Fatal("expected non-success response to be reported")
	}
}
