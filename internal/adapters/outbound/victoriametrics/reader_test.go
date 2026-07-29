package victoriametrics

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"argus/internal/models"
)

func TestReaderBuildsScopedAvailabilityAggregates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method: %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		query := r.Form.Get("query")
		if !strings.Contains(query, `argus_project_id="12"`) {
			t.Fatalf("missing project scope: %s", query)
		}
		value := "100"
		if strings.Contains(query, "http_status_code") {
			value = "3"
		}
		if strings.Contains(query, "timestamp(") {
			value = "1710000000"
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":["0","` + value + `"]}]}}`))
	}))
	defer server.Close()
	reader, err := NewReader(server.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	aggregate, err := reader.AggregateSLO(context.Background(), models.SLODefinition{ProjectID: 12, SLIKind: "availability", WindowSeconds: 3600}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if aggregate.TotalEvents != 100 || aggregate.GoodEvents != 97 || aggregate.ObservedAt == nil {
		t.Fatalf("unexpected aggregate: %+v", aggregate)
	}
}

func TestReaderReturnsNoDataForEmptyVector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[]}}`))
	}))
	defer server.Close()
	reader, _ := NewReader(server.URL, time.Second)
	aggregate, err := reader.AggregateSLO(context.Background(), models.SLODefinition{ProjectID: 1, SLIKind: "availability", WindowSeconds: 60}, time.Now())
	if err != nil || aggregate.TotalEvents != 0 || aggregate.ObservedAt != nil {
		t.Fatalf("unexpected no-data aggregate: %+v, %v", aggregate, err)
	}
}
