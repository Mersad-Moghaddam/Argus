package notifier

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"argus/internal/models"
)

func TestHTTPNotifier_PropagatesFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	n := NewHTTPNotifier()
	err := n.Notify(context.Background(), []models.AlertChannel{{Name: "broken", ChannelType: "webhook", Target: server.URL}}, []byte(`{"ok":true}`))
	if err == nil {
		t.Fatal("expected notify to return error on non-2xx delivery")
	}
}
