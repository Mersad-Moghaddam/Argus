package api

import (
	"context"
	"errors"
	"time"

	"argus/internal/application"
	"argus/internal/models"
	metricscollector "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracecollector "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TelemetryGRPCServer reuses the OTLP/HTTP policy and stores only the same
// bounded, credential-attributed diagnostics and sanitized metric samples.
type telemetryGRPCCore struct {
	handler *TelemetryIngestHandler
}
type TelemetryMetricsGRPCServer struct {
	metricscollector.UnimplementedMetricsServiceServer
	core *telemetryGRPCCore
}
type TelemetryTracesGRPCServer struct {
	tracecollector.UnimplementedTraceServiceServer
	core *telemetryGRPCCore
}

func NewTelemetryGRPCServers(handler *TelemetryIngestHandler) (*TelemetryMetricsGRPCServer, *TelemetryTracesGRPCServer) {
	core := &telemetryGRPCCore{handler: handler}
	return &TelemetryMetricsGRPCServer{core: core}, &TelemetryTracesGRPCServer{core: core}
}

func (s *TelemetryMetricsGRPCServer) Export(ctx context.Context, request *metricscollector.ExportMetricsServiceRequest) (*metricscollector.ExportMetricsServiceResponse, error) {
	credential, err := s.core.authorize(ctx, "metrics:write")
	if err != nil {
		return nil, err
	}
	records, err := metricIngressRecords(credential, request.GetResourceMetrics())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	samples, err := prometheusHTTPSamples(credential, request.GetResourceMetrics())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err = s.core.handler.metricSink.Write(ctx, samples); err != nil {
		return nil, status.Error(codes.Unavailable, "telemetry metrics storage is temporarily unavailable")
	}
	if err = s.core.handler.recordContext(ctx, credential, records); err != nil {
		return nil, status.Error(codes.Unavailable, "telemetry ingestion is temporarily unavailable")
	}
	return &metricscollector.ExportMetricsServiceResponse{}, nil
}

func (s *TelemetryTracesGRPCServer) Export(ctx context.Context, request *tracecollector.ExportTraceServiceRequest) (*tracecollector.ExportTraceServiceResponse, error) {
	credential, err := s.core.authorize(ctx, "traces:write")
	if err != nil {
		return nil, err
	}
	records, err := traceIngressRecords(credential, request.GetResourceSpans())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if err = s.core.handler.recordContext(ctx, credential, records); err != nil {
		return nil, status.Error(codes.Unavailable, "telemetry ingestion is temporarily unavailable")
	}
	return &tracecollector.ExportTraceServiceResponse{}, nil
}

func (s *telemetryGRPCCore) authorize(ctx context.Context, scope string) (*models.TelemetryCredential, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid telemetry credentials")
	}
	values := md.Get("authorization")
	if len(values) != 1 {
		return nil, status.Error(codes.Unauthenticated, "invalid telemetry credentials")
	}
	raw, ok := bearerCredential(values[0])
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid telemetry credentials")
	}
	credential, err := s.handler.service.AuthenticateTelemetryCredential(ctx, raw)
	if errors.Is(err, application.ErrTelemetryCredentialNotFound) {
		return nil, status.Error(codes.Unauthenticated, "invalid telemetry credentials")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "telemetry authentication is temporarily unavailable")
	}
	if !credentialHasScope(credential, scope) {
		return nil, status.Error(codes.PermissionDenied, "telemetry credential scope is insufficient")
	}
	if !s.handler.limits.Allow(credential.ID, credential.RateLimitPerMinute, time.Now().UTC()) {
		return nil, status.Error(codes.ResourceExhausted, "telemetry credential rate limit exceeded")
	}
	return credential, nil
}
