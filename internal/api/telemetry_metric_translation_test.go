package api

import (
	"testing"
	"time"

	"argus/internal/models"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
)

func TestPrometheusHTTPSamplesTranslatesOnlyBoundedHistogramLabels(t *testing.T) {
	sum := 250.0
	timestamp := uint64(time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC).UnixNano())
	point := &metricspb.HistogramDataPoint{
		Attributes:   []*commonpb.KeyValue{stringKV("http.request.method", "GET"), stringKV("http.route", "/orders/{orderId}"), intKV("http.response.status_code", 200)},
		TimeUnixNano: timestamp, Count: 3, Sum: &sum, BucketCounts: []uint64{2, 1}, ExplicitBounds: []float64{100},
	}
	metric := &metricspb.Metric{Name: "http.server.request.duration", Unit: "ms", Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{DataPoints: []*metricspb.HistogramDataPoint{point}}}}
	groups := []*metricspb.ResourceMetrics{{
		Resource:     &resourcepb.Resource{Attributes: []*commonpb.KeyValue{stringKV("service.name", "checkout"), stringKV("deployment.environment.name", "production")}},
		ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{metric}}},
	}}
	samples, err := prometheusHTTPSamples(&models.TelemetryCredential{ProjectID: 7, EnvironmentID: 11}, groups)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(samples) != 4 {
		t.Fatalf("expected 4 samples, got %#v", samples)
	}
	if samples[0].Name != argusHTTPDurationMetric+"_bucket" || samples[0].Labels["le"] != "0.1" || samples[0].Value != 2 {
		t.Fatalf("unexpected first bucket: %+v", samples[0])
	}
	if samples[1].Labels["le"] != "+Inf" || samples[1].Value != 3 {
		t.Fatalf("unexpected infinity bucket: %+v", samples[1])
	}
	if samples[2].Name != argusHTTPDurationMetric+"_count" || samples[2].Value != 3 || samples[3].Name != argusHTTPDurationMetric+"_sum" || samples[3].Value != .25 {
		t.Fatalf("unexpected aggregate samples: %+v", samples)
	}
	for _, sample := range samples {
		if sample.Labels["argus_project_id"] != "7" || sample.Labels["argus_environment_id"] != "11" || sample.Labels["service_name"] != "checkout" || sample.Labels["http_route"] != "/orders/{orderId}" || sample.Labels["http_status_code"] != "200" {
			t.Fatalf("incorrect bounded labels: %+v", sample.Labels)
		}
	}
}

func TestPrometheusHTTPSamplesDropsUnsafeOrIrrelevantSignals(t *testing.T) {
	timestamp := uint64(time.Now().UTC().UnixNano())
	unsafe := &metricspb.Metric{Name: "http.server.request.duration", Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{DataPoints: []*metricspb.HistogramDataPoint{{
		Attributes: []*commonpb.KeyValue{stringKV("http.request.method", "GET"), stringKV("http.route", "/orders/12345"), intKV("http.response.status_code", 200)}, TimeUnixNano: timestamp, Count: 1,
	}}}}}
	irrelevant := &metricspb.Metric{Name: "process.runtime.go.gc_count", Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{}}}
	groups := []*metricspb.ResourceMetrics{{ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{unsafe, irrelevant}}}}}
	samples, err := prometheusHTTPSamples(&models.TelemetryCredential{ProjectID: 1, EnvironmentID: 1}, groups)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	if len(samples) != 0 {
		t.Fatalf("unsafe and irrelevant telemetry must not become metric series: %+v", samples)
	}
}

func stringKV(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}}}
}

func intKV(key string, value int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: value}}}
}
