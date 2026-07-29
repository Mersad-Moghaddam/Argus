package api_test

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"argus/internal/api"
	"argus/internal/application"

	"github.com/gofiber/fiber/v2"
	metricscollector "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracecollector "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

func TestOTLPGRPCTransportIngestsMetrics(t *testing.T) {
	a := newTestAPI(t)
	userID, token := a.register(t, "otlp-grpc-transport@example.com")
	project := a.createProject(t, token, "OTLP gRPC transport")
	environments, err := a.service.ListProjectEnvironments(context.Background(), project.ID)
	if err != nil || len(environments) != 1 {
		t.Fatalf("default environment: %v %+v", err, environments)
	}
	issued, err := a.service.CreateTelemetryCredential(context.Background(), project.ID, userID, application.CreateTelemetryCredentialInput{Name: "grpc transport", EnvironmentID: environments[0].ID})
	if err != nil {
		t.Fatal(err)
	}

	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	metricsServer, tracesServer := api.NewTelemetryGRPCServers(api.NewTelemetryIngestHandler(a.service))
	metricscollector.RegisterMetricsServiceServer(server, metricsServer)
	tracecollector.RegisterTraceServiceServer(server, tracesServer)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := metricscollector.NewMetricsServiceClient(conn)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+issued.Token)
	request := &metricscollector.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{{
		Resource:     resourceWithMetadata("grpc-client", "production", "https://forged.invalid"),
		ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{{Name: "rpc.server.duration"}}}},
	}}}
	if _, err = client.Export(ctx, request); err != nil {
		t.Fatalf("transport metrics export: %v", err)
	}
	records, err := a.service.ListTelemetryIngress(context.Background(), project.ID, 10)
	if err != nil || len(records) != 1 || records[0].ProjectID != project.ID || records[0].EnvironmentID != environments[0].ID || records[0].ServiceName != "grpc-client" {
		t.Fatalf("transport record: %#v %v", records, err)
	}
}

func TestOTLPGRPCIngestUsesCredentialBoundAttribution(t *testing.T) {
	a := newTestAPI(t)
	userID, token := a.register(t, "otlp-grpc@example.com")
	project := a.createProject(t, token, "OTLP gRPC")
	environments, _ := a.service.ListProjectEnvironments(context.Background(), project.ID)
	issued, err := a.service.CreateTelemetryCredential(context.Background(), project.ID, userID, application.CreateTelemetryCredentialInput{Name: "grpc", EnvironmentID: environments[0].ID, RateLimitPerMinute: 2})
	if err != nil {
		t.Fatal(err)
	}
	metricsServer, _ := api.NewTelemetryGRPCServers(api.NewTelemetryIngestHandler(a.service))
	req := &metricscollector.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{{Resource: resourceWithMetadata("grpc-checkout", "production", "https://forged.invalid"), ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{{Name: "http.server.duration"}}}}}}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+issued.Token))
	if _, err = metricsServer.Export(ctx, req); err != nil {
		t.Fatalf("grpc metrics export: %v", err)
	}
	records, err := a.service.ListTelemetryIngress(context.Background(), project.ID, 10)
	if err != nil || len(records) != 1 || records[0].ProjectID != project.ID || records[0].EnvironmentID != environments[0].ID || records[0].ServiceName != "grpc-checkout" {
		t.Fatalf("credential-bound grpc record: %#v %v", records, err)
	}
	if _, err = metricsServer.Export(context.Background(), req); status.Code(err) != codes.Unauthenticated {
		t.Fatalf("missing credential code = %v (%v)", status.Code(err), err)
	}
}

func TestOTLPHTTPIngestUsesCredentialBoundAttribution(t *testing.T) {
	a := newTestAPI(t)
	userID, token := a.register(t, "otlp@example.com")
	project := a.createProject(t, token, "OTLP")
	environments, err := a.service.ListProjectEnvironments(context.Background(), project.ID)
	if err != nil || len(environments) != 1 {
		t.Fatalf("default environment: %v %+v", err, environments)
	}
	issued, err := a.service.CreateTelemetryCredential(context.Background(), project.ID, userID, application.CreateTelemetryCredentialInput{
		Name: "production collector", EnvironmentID: environments[0].ID, RateLimitPerMinute: 1,
	})
	if err != nil {
		t.Fatalf("issue telemetry credential: %v", err)
	}

	metricsPayload, err := proto.Marshal(&metricscollector.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{{
		Resource:     resourceWithMetadata("checkout", "production", "https://secret.invalid/private"),
		ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{{Name: "http.server.duration"}}}},
	}}})
	if err != nil {
		t.Fatalf("marshal metrics: %v", err)
	}
	response := a.otlpRequest(t, "/v1/metrics", issued.Token, metricsPayload, "application/x-protobuf")
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("metrics ingest: expected 200, got %d (%s)", response.StatusCode, bodyString(t, response))
	}
	_ = response.Body.Close()

	// The fake sender tries to forge project and environment identifiers in the
	// resource. The receiver ignores them and uses the credential binding.
	records, err := a.service.ListTelemetryIngress(context.Background(), project.ID, 10)
	if err != nil || len(records) != 1 {
		t.Fatalf("receiver record: %v %+v", err, records)
	}
	record := records[0]
	if record.ProjectID != project.ID || record.EnvironmentID != environments[0].ID || record.CredentialID != issued.Credential.ID {
		t.Fatalf("forged resource metadata changed attribution: %+v", record)
	}
	if record.ServiceName != "checkout" || record.DeploymentEnvironment != "production" || record.ItemCount != 1 || record.SignalType != "metrics" {
		t.Fatalf("unexpected bounded diagnostics: %+v", record)
	}

	limited := a.otlpRequest(t, "/v1/metrics", issued.Token, metricsPayload, "application/x-protobuf")
	if limited.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("expected credential rate limit, got %d (%s)", limited.StatusCode, bodyString(t, limited))
	}
	_ = limited.Body.Close()
}

func TestOTLPHTTPIngestRejectsBadCredentialsAndContent(t *testing.T) {
	a := newTestAPI(t)
	if response := a.otlpRequest(t, "/v1/metrics", "not-a-token", []byte("not-protobuf"), "application/x-protobuf"); response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("unknown credential: expected 401, got %d", response.StatusCode)
	} else {
		_ = response.Body.Close()
	}

	_, token := a.register(t, "otlp-content@example.com")
	project := a.createProject(t, token, "OTLP content")
	environments, _ := a.service.ListProjectEnvironments(context.Background(), project.ID)
	issued, err := a.service.CreateTelemetryCredential(context.Background(), project.ID, 1, application.CreateTelemetryCredentialInput{Name: "collector", EnvironmentID: environments[0].ID})
	if err != nil {
		t.Fatalf("issue telemetry credential: %v", err)
	}
	if response := a.otlpRequest(t, "/v1/traces", issued.Token, []byte("{}"), "application/json"); response.StatusCode != fiber.StatusUnsupportedMediaType {
		t.Fatalf("json content type: expected 415, got %d", response.StatusCode)
	} else {
		_ = response.Body.Close()
	}
	if response := a.otlpRequest(t, "/v1/traces", issued.Token, []byte("not-protobuf"), "application/x-protobuf"); response.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("malformed protobuf: expected 400, got %d", response.StatusCode)
	} else {
		_ = response.Body.Close()
	}
}

func TestOTLPHTTPTraceIngestRecordsOnlyAllowlistedResourceFields(t *testing.T) {
	a := newTestAPI(t)
	userID, token := a.register(t, "otlp-trace@example.com")
	project := a.createProject(t, token, "OTLP trace")
	environments, _ := a.service.ListProjectEnvironments(context.Background(), project.ID)
	issued, err := a.service.CreateTelemetryCredential(context.Background(), project.ID, userID, application.CreateTelemetryCredentialInput{Name: "collector", EnvironmentID: environments[0].ID})
	if err != nil {
		t.Fatalf("issue credential: %v", err)
	}
	payload, err := proto.Marshal(&tracecollector.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource:   resourceWithMetadata("payments", "staging", "Bearer top-secret"),
		ScopeSpans: []*tracepb.ScopeSpans{{Spans: []*tracepb.Span{{Name: "/orders/123?token=secret"}}}},
	}}})
	if err != nil {
		t.Fatalf("marshal traces: %v", err)
	}
	response := a.otlpRequest(t, "/v1/traces", issued.Token, payload, "application/protobuf")
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("trace ingest: expected 200, got %d (%s)", response.StatusCode, bodyString(t, response))
	}
	_ = response.Body.Close()
	records, _ := a.service.ListTelemetryIngress(context.Background(), project.ID, 10)
	if len(records) != 1 || records[0].SignalType != "traces" || records[0].ServiceName != "payments" || records[0].DeploymentEnvironment != "staging" {
		t.Fatalf("unexpected trace audit record: %+v", records)
	}
}

func (a *testAPI) otlpRequest(t *testing.T, path, credential string, body []byte, contentType string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set(fiber.HeaderContentType, contentType)
	req.Header.Set(fiber.HeaderAuthorization, "Bearer "+credential)
	response, err := a.app.Test(req, -1)
	if err != nil {
		t.Fatalf("OTLP request: %v", err)
	}
	return response
}

func resourceWithMetadata(serviceName, deploymentEnvironment, sensitive string) *resourcepb.Resource {
	return &resourcepb.Resource{Attributes: []*commonpb.KeyValue{
		stringAttribute("service.name", serviceName),
		stringAttribute("deployment.environment.name", deploymentEnvironment),
		stringAttribute("http.url", sensitive),
		stringAttribute("argus.project.id", "999999"),
		stringAttribute("argus.environment.id", "999999"),
	}}
}

func stringAttribute(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: key, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}}}
}
