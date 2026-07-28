package api

import (
	"errors"
	"mime"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"argus/internal/application"
	"argus/internal/models"

	"github.com/gofiber/fiber/v2"
	metricscollector "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracecollector "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

const (
	maxOTLPPayloadBytes    = 4 * 1024 * 1024
	maxOTLPResourceGroups  = 100
	maxOTLPItemsPerRequest = 10_000
)

// TelemetryIngestHandler is the OTLP/HTTP boundary. It translates an opaque
// credential into authoritative tenant attribution and stores only bounded,
// allowlisted receiver diagnostics. It deliberately does not persist raw
// attributes, measurements, trace identifiers, span names, or payloads.
type TelemetryIngestHandler struct {
	service *application.Service
	limits  *telemetryRateLimiter
}

func NewTelemetryIngestHandler(service *application.Service) *TelemetryIngestHandler {
	return &TelemetryIngestHandler{service: service, limits: newTelemetryRateLimiter(4096)}
}

func RegisterTelemetryIngestRoutes(app fiber.Router, h *TelemetryIngestHandler) {
	app.Post("/v1/metrics", h.ExportMetrics)
	app.Post("/v1/traces", h.ExportTraces)
}

func (h *TelemetryIngestHandler) ExportMetrics(c *fiber.Ctx) error {
	credential, ok := h.authorize(c, "metrics:write")
	if !ok {
		return nil
	}
	if !isOTLPProtobuf(c.Get(fiber.HeaderContentType)) {
		return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{"error": "OTLP HTTP requires application/x-protobuf"})
	}
	if len(c.Body()) > maxOTLPPayloadBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "telemetry payload is too large"})
	}
	var request metricscollector.ExportMetricsServiceRequest
	if err := proto.Unmarshal(c.Body(), &request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid OTLP protobuf payload"})
	}
	records, err := metricIngressRecords(credential, request.GetResourceMetrics())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err = h.record(c, credential, records); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "telemetry ingestion is temporarily unavailable"})
	}
	body, marshalErr := proto.Marshal(&metricscollector.ExportMetricsServiceResponse{})
	if marshalErr != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	c.Type("application/x-protobuf")
	return c.Status(fiber.StatusOK).Send(body)
}

func (h *TelemetryIngestHandler) ExportTraces(c *fiber.Ctx) error {
	credential, ok := h.authorize(c, "traces:write")
	if !ok {
		return nil
	}
	if !isOTLPProtobuf(c.Get(fiber.HeaderContentType)) {
		return c.Status(fiber.StatusUnsupportedMediaType).JSON(fiber.Map{"error": "OTLP HTTP requires application/x-protobuf"})
	}
	if len(c.Body()) > maxOTLPPayloadBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "telemetry payload is too large"})
	}
	var request tracecollector.ExportTraceServiceRequest
	if err := proto.Unmarshal(c.Body(), &request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid OTLP protobuf payload"})
	}
	records, err := traceIngressRecords(credential, request.GetResourceSpans())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err = h.record(c, credential, records); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "telemetry ingestion is temporarily unavailable"})
	}
	body, marshalErr := proto.Marshal(&tracecollector.ExportTraceServiceResponse{})
	if marshalErr != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}
	c.Type("application/x-protobuf")
	return c.Status(fiber.StatusOK).Send(body)
}

func (h *TelemetryIngestHandler) authorize(c *fiber.Ctx, requiredScope string) (*models.TelemetryCredential, bool) {
	if len(c.Body()) > maxOTLPPayloadBytes {
		_ = c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "telemetry payload is too large"})
		return nil, false
	}
	raw, ok := bearerCredential(c.Get(fiber.HeaderAuthorization))
	if !ok {
		_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid telemetry credentials"})
		return nil, false
	}
	credential, err := h.service.AuthenticateTelemetryCredential(c.UserContext(), raw)
	if errors.Is(err, application.ErrTelemetryCredentialNotFound) {
		_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid telemetry credentials"})
		return nil, false
	}
	if err != nil {
		_ = c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "telemetry authentication is temporarily unavailable"})
		return nil, false
	}
	if !credentialHasScope(credential, requiredScope) {
		_ = c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "telemetry credential scope is insufficient"})
		return nil, false
	}
	if !h.limits.Allow(credential.ID, credential.RateLimitPerMinute, time.Now().UTC()) {
		_ = c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "telemetry credential rate limit exceeded"})
		return nil, false
	}
	return credential, true
}

func (h *TelemetryIngestHandler) record(c *fiber.Ctx, credential *models.TelemetryCredential, records []models.TelemetryIngressRecord) error {
	for _, record := range records {
		if err := h.service.RecordTelemetryIngress(c.UserContext(), credential, record); err != nil {
			return err
		}
	}
	return nil
}

func metricIngressRecords(credential *models.TelemetryCredential, groups []*metricspb.ResourceMetrics) ([]models.TelemetryIngressRecord, error) {
	if len(groups) > maxOTLPResourceGroups {
		return nil, errors.New("too many OTLP resource groups")
	}
	records := make([]models.TelemetryIngressRecord, 0, len(groups))
	total := 0
	for _, group := range groups {
		count := 0
		for _, scope := range group.GetScopeMetrics() {
			count += len(scope.GetMetrics())
		}
		total += count
		if total > maxOTLPItemsPerRequest {
			return nil, errors.New("too many OTLP metric items")
		}
		serviceName, deploymentEnvironment := allowedResourceMetadata(group.GetResource())
		records = append(records, boundIngressRecord(credential, "metrics", serviceName, deploymentEnvironment, count))
	}
	return records, nil
}

func traceIngressRecords(credential *models.TelemetryCredential, groups []*tracepb.ResourceSpans) ([]models.TelemetryIngressRecord, error) {
	if len(groups) > maxOTLPResourceGroups {
		return nil, errors.New("too many OTLP resource groups")
	}
	records := make([]models.TelemetryIngressRecord, 0, len(groups))
	total := 0
	for _, group := range groups {
		count := 0
		for _, scope := range group.GetScopeSpans() {
			count += len(scope.GetSpans())
		}
		total += count
		if total > maxOTLPItemsPerRequest {
			return nil, errors.New("too many OTLP span items")
		}
		serviceName, deploymentEnvironment := allowedResourceMetadata(group.GetResource())
		records = append(records, boundIngressRecord(credential, "traces", serviceName, deploymentEnvironment, count))
	}
	return records, nil
}

func boundIngressRecord(credential *models.TelemetryCredential, signalType, serviceName, deploymentEnvironment string, itemCount int) models.TelemetryIngressRecord {
	return models.TelemetryIngressRecord{
		ProjectID: credential.ProjectID, EnvironmentID: credential.EnvironmentID, CredentialID: credential.ID,
		SignalType: signalType, ServiceName: serviceName, DeploymentEnvironment: deploymentEnvironment, ItemCount: itemCount,
	}
}

func allowedResourceMetadata(resource *resourcepb.Resource) (serviceName, deploymentEnvironment string) {
	for _, attribute := range resource.GetAttributes() {
		switch attribute.GetKey() {
		case "service.name":
			serviceName = normalizeTelemetryAttribute(attribute.GetValue())
		case "deployment.environment.name":
			deploymentEnvironment = normalizeTelemetryAttribute(attribute.GetValue())
		}
	}
	return serviceName, deploymentEnvironment
}

func normalizeTelemetryAttribute(value *commonpb.AnyValue) string {
	valueString := strings.TrimSpace(value.GetStringValue())
	if valueString == "" || !utf8.ValidString(valueString) || len(valueString) > 160 {
		return ""
	}
	for _, char := range valueString {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || strings.ContainsRune("._:/-", char)) {
			return ""
		}
	}
	return valueString
}

func isOTLPProtobuf(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && (mediaType == "application/x-protobuf" || mediaType == "application/protobuf")
}

func bearerCredential(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func credentialHasScope(credential *models.TelemetryCredential, required string) bool {
	for _, scope := range strings.Split(credential.Scopes, ",") {
		if strings.TrimSpace(scope) == required {
			return true
		}
	}
	return false
}

type telemetryRateWindow struct {
	minute time.Time
	count  int
	seenAt time.Time
}

type telemetryRateLimiter struct {
	mu         sync.Mutex
	maxEntries int
	entries    map[int64]telemetryRateWindow
}

func newTelemetryRateLimiter(maxEntries int) *telemetryRateLimiter {
	return &telemetryRateLimiter{maxEntries: maxEntries, entries: map[int64]telemetryRateWindow{}}
}

func (l *telemetryRateLimiter) Allow(credentialID int64, limit int, now time.Time) bool {
	if limit <= 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	windowStart := now.Truncate(time.Minute)
	entry, exists := l.entries[credentialID]
	if !exists && len(l.entries) >= l.maxEntries {
		l.evictOldest()
	}
	if entry.minute != windowStart {
		entry.minute, entry.count = windowStart, 0
	}
	if entry.count >= limit {
		entry.seenAt = now
		l.entries[credentialID] = entry
		return false
	}
	entry.count++
	entry.seenAt = now
	l.entries[credentialID] = entry
	return true
}

func (l *telemetryRateLimiter) evictOldest() {
	var oldestID int64
	var oldest time.Time
	for id, entry := range l.entries {
		if oldest.IsZero() || entry.seenAt.Before(oldest) {
			oldestID, oldest = id, entry.seenAt
		}
	}
	delete(l.entries, oldestID)
}
