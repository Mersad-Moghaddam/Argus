package observability

import (
	"context"
	"time"
)

// MetricSample is a sanitized Prometheus-compatible sample. The OTLP ingress
// boundary creates it only from its explicit metric and attribute allowlists.
type MetricSample struct {
	Name      string
	Labels    map[string]string
	Value     float64
	Timestamp time.Time
}

// MetricSink persists sanitized metrics outside the transactional MySQL
// control plane.
type MetricSink interface {
	Write(ctx context.Context, samples []MetricSample) error
}

type noopMetricSink struct{}

func (noopMetricSink) Write(context.Context, []MetricSample) error { return nil }

func NoopMetricSink() MetricSink { return noopMetricSink{} }
