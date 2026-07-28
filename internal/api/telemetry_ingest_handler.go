package api

import (
	"errors"
	"math"
	"mime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"argus/internal/application"
	"argus/internal/domain"
	"argus/internal/models"
	"argus/internal/observability"

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
	service    *application.Service
	limits     *telemetryRateLimiter
	metricSink observability.MetricSink
}

func NewTelemetryIngestHandler(service *application.Service, sinks ...observability.MetricSink) *TelemetryIngestHandler {
	sink := observability.NoopMetricSink()
	if len(sinks) > 0 && sinks[0] != nil {
		sink = sinks[0]
	}
	return &TelemetryIngestHandler{service: service, limits: newTelemetryRateLimiter(4096), metricSink: sink}
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
	samples, err := prometheusHTTPSamples(credential, request.GetResourceMetrics())
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err = h.metricSink.Write(c.UserContext(), samples); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "telemetry metrics storage is temporarily unavailable"})
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

const (
	argusHTTPDurationMetric        = "argus_http_server_request_duration_seconds"
	maxPrometheusSamplesPerRequest = 20_000
)

// prometheusHTTPSamples converts only HTTP server duration histograms into
// Argus-owned, bounded Prometheus series. Every source label is allowlisted;
// arbitrary OTLP metric names, attributes and exemplar values are ignored.
func prometheusHTTPSamples(credential *models.TelemetryCredential, groups []*metricspb.ResourceMetrics) ([]observability.MetricSample, error) {
	samples := make([]observability.MetricSample, 0)
	for _, group := range groups {
		serviceName, deploymentEnvironment := allowedResourceMetadata(group.GetResource())
		for _, scope := range group.GetScopeMetrics() {
			for _, metric := range scope.GetMetrics() {
				if !isHTTPDurationHistogram(metric) {
					continue
				}
				factor, ok := durationUnitFactor(metric.GetUnit())
				if !ok {
					continue
				}
				for _, point := range metric.GetHistogram().GetDataPoints() {
					labels, timestamp, ok := safeHTTPMetricLabels(credential, serviceName, deploymentEnvironment, point.GetAttributes(), point.GetTimeUnixNano())
					if !ok {
						continue
					}
					pointSamples, err := histogramSamples(labels, timestamp, point, factor)
					if err != nil {
						return nil, err
					}
					samples = append(samples, pointSamples...)
					if len(samples) > maxPrometheusSamplesPerRequest {
						return nil, errors.New("too many Prometheus samples after telemetry conversion")
					}
				}
			}
		}
	}
	return samples, nil
}

func isHTTPDurationHistogram(metric *metricspb.Metric) bool {
	if metric.GetHistogram() == nil {
		return false
	}
	switch metric.GetName() {
	case "http.server.request.duration", "http.server.duration", "http.server.request.duration.seconds":
		return true
	default:
		return false
	}
}

func durationUnitFactor(unit string) (float64, bool) {
	switch strings.TrimSpace(unit) {
	case "", "s", "sec", "seconds":
		return 1, true
	case "ms", "millisecond", "milliseconds":
		return 0.001, true
	case "us", "µs", "microsecond", "microseconds":
		return 0.000001, true
	case "ns", "nanosecond", "nanoseconds":
		return 0.000000001, true
	default:
		return 0, false
	}
}

func histogramSamples(labels map[string]string, timestamp time.Time, point *metricspb.HistogramDataPoint, factor float64) ([]observability.MetricSample, error) {
	if point.GetTimeUnixNano() == 0 || point.GetCount() == 0 || point.GetCount() > math.MaxInt64 {
		return nil, nil
	}
	counts, bounds := point.GetBucketCounts(), point.GetExplicitBounds()
	if len(counts) != 0 && len(counts) != len(bounds)+1 {
		return nil, errors.New("invalid OTLP histogram bucket shape")
	}
	samples := make([]observability.MetricSample, 0, len(counts)+2)
	if len(counts) > 0 {
		var cumulative uint64
		for index, count := range counts {
			if math.MaxUint64-cumulative < count {
				return nil, errors.New("OTLP histogram bucket count overflow")
			}
			cumulative += count
			bucketLabels := copyLabels(labels)
			if index < len(bounds) {
				if !finite(bounds[index]) || bounds[index] < 0 {
					return nil, errors.New("invalid OTLP histogram boundary")
				}
				bucketLabels["le"] = strconv.FormatFloat(bounds[index]*factor, 'g', -1, 64)
			} else {
				bucketLabels["le"] = "+Inf"
			}
			samples = append(samples, observability.MetricSample{Name: argusHTTPDurationMetric + "_bucket", Labels: bucketLabels, Value: float64(cumulative), Timestamp: timestamp})
		}
		if cumulative != point.GetCount() {
			return nil, errors.New("invalid OTLP histogram bucket total")
		}
	}
	samples = append(samples, observability.MetricSample{Name: argusHTTPDurationMetric + "_count", Labels: copyLabels(labels), Value: float64(point.GetCount()), Timestamp: timestamp})
	if point.Sum != nil && finite(point.GetSum()) && point.GetSum() >= 0 {
		samples = append(samples, observability.MetricSample{Name: argusHTTPDurationMetric + "_sum", Labels: copyLabels(labels), Value: point.GetSum() * factor, Timestamp: timestamp})
	}
	return samples, nil
}

func safeHTTPMetricLabels(credential *models.TelemetryCredential, serviceName, deploymentEnvironment string, attributes []*commonpb.KeyValue, timeUnixNano uint64) (map[string]string, time.Time, bool) {
	if timeUnixNano == 0 || timeUnixNano > math.MaxInt64 {
		return nil, time.Time{}, false
	}
	values := map[string]string{}
	for _, attribute := range attributes {
		switch attribute.GetKey() {
		case "http.request.method", "http.method":
			values["http_method"] = strings.ToUpper(normalizeTelemetryAttribute(attribute.GetValue()))
		case "http.route":
			values["http_route"] = safeRouteTemplate(attribute.GetValue())
		case "http.response.status_code", "http.status_code":
			values["http_status_code"] = safeStatusCode(attribute.GetValue())
		}
	}
	if values["http_method"] == "" || values["http_route"] == "" || values["http_status_code"] == "" {
		return nil, time.Time{}, false
	}
	labels := map[string]string{
		"argus_project_id":       strconv.FormatInt(credential.ProjectID, 10),
		"argus_environment_id":   strconv.FormatInt(credential.EnvironmentID, 10),
		"service_name":           fallbackTelemetryLabel(serviceName),
		"deployment_environment": fallbackTelemetryLabel(deploymentEnvironment),
		"http_method":            values["http_method"], "http_route": values["http_route"], "http_status_code": values["http_status_code"],
	}
	return labels, time.Unix(0, int64(timeUnixNano)).UTC(), true
}

func safeRouteTemplate(value *commonpb.AnyValue) string {
	raw := strings.TrimSpace(value.GetStringValue())
	if raw == "" || len(raw) > 1024 || !utf8.ValidString(raw) {
		return ""
	}
	route, err := domain.NormalizeRouteTemplate(raw)
	if err != nil || route == "" {
		return ""
	}
	for _, segment := range strings.Split(route, "/") {
		if len(segment) >= 4 {
			allDigits := true
			for _, char := range segment {
				if char < '0' || char > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return ""
			}
		}
	}
	return route
}

func safeStatusCode(value *commonpb.AnyValue) string {
	var status int64
	if value.GetIntValue() != 0 {
		status = value.GetIntValue()
	} else {
		parsed, err := strconv.ParseInt(normalizeTelemetryAttribute(value), 10, 16)
		if err != nil {
			return ""
		}
		status = parsed
	}
	if status < 100 || status > 599 {
		return ""
	}
	return strconv.FormatInt(status, 10)
}

func fallbackTelemetryLabel(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}
func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func copyLabels(source map[string]string) map[string]string {
	copy := make(map[string]string, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
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
