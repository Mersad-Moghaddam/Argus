package recovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRecoveryDeliveryDoesNotFollowRedirects(t *testing.T) {
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
	delivery, err := NewWebhookDelivery(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	client.CheckRedirect = delivery.client.CheckRedirect
	delivery.client = client
	if err = delivery.DeliverPasswordRecovery(context.Background(), "user@example.test", "secret", time.Now().Add(time.Hour)); err == nil {
		t.Fatal("expected redirecting delivery to be rejected")
	}
	if followed {
		t.Fatal("recovery token delivery followed a redirect")
	}
}
