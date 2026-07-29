package victoriametrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"argus/internal/observability"
)

func TestWriterImportsNewlineDelimitedSamples(t *testing.T) {
	var contentType, payload string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/import" {
			t.Errorf("path = %s", r.URL.Path)
		}
		contentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		payload = string(body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	writer, err := NewWriter(server.URL, time.Second)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	err = writer.Write(context.Background(), []observability.MetricSample{{Name: "argus_test_total", Labels: map[string]string{"service_name": "checkout"}, Value: 3, Timestamp: time.Unix(1_700_000_000, 0)}})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if contentType != "application/stream+json" || !strings.Contains(payload, `"__name__":"argus_test_total"`) || !strings.Contains(payload, `"service_name":"checkout"`) {
		t.Fatalf("unexpected import request: content-type=%q payload=%q", contentType, payload)
	}
}

func TestWriterReturnsBackendFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "full", http.StatusServiceUnavailable) }))
	defer server.Close()
	writer, _ := NewWriter(server.URL, time.Second)
	err := writer.Write(context.Background(), []observability.MetricSample{{Name: "argus_test_total", Timestamp: time.Now().UTC()}})
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected backend status error, got %v", err)
	}
}
