package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// Telemetry owns Argus's process-level OpenTelemetry providers. Exporters are
// deliberately not configured here: production export is supplied by the
// authenticated OTLP pipeline, while these providers make instrumentation safe
// and no-op-free in every deployment mode.
type Telemetry struct {
	traces  *trace.TracerProvider
	metrics *metric.MeterProvider
}

func NewTelemetry(serviceName, serviceVersion string) *Telemetry {
	res := resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName(serviceName), semconv.ServiceVersion(serviceVersion))
	traces := trace.NewTracerProvider(trace.WithResource(res))
	metrics := metric.NewMeterProvider(metric.WithResource(res))
	otel.SetTracerProvider(traces)
	otel.SetMeterProvider(metrics)
	return &Telemetry{traces: traces, metrics: metrics}
}

func (t *Telemetry) Shutdown(ctx context.Context) error {
	if err := t.metrics.Shutdown(ctx); err != nil {
		return err
	}
	return t.traces.Shutdown(ctx)
}
