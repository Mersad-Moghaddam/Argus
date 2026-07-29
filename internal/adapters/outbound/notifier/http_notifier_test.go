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
	if err := n.Notify(context.Background(), []models.AlertChannel{{ChannelType: "webhook", Target: server.URL, Enabled: true}}, []byte(`{"event":"incident"}`)); err != nil {
		t.Fatal(err)
	}
	if followed {
		t.Fatal("notification payload followed a redirect")
	}
}
