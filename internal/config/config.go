package config

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"time"

	"argus/internal/secrets"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr string
	MySQLDSN string

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	SchedulerInterval   time.Duration
	WorkerConcurrency   int
	QueueCriticalWeight int
	QueueDefaultWeight  int
	DueCheckBatchSize   int
	APIKey              string
	// AuthCookieSecure must be enabled behind TLS in production. It remains
	// configurable so local HTTP development and test clients keep working.
	AuthCookieSecure bool
	// RecoveryDeliveryURL is an operator-configured HTTPS webhook that receives
	// one-time reset tokens for delivery to a verified account address. Empty
	// disables delivery without exposing a token through the public API.
	RecoveryDeliveryURL      string
	RecoveryDeliveryTimeout  time.Duration
	RouteSecretEncryptionKey []byte

	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	// MetricsBackendURL is internal-only VictoriaMetrics access for sanitized
	// OTLP samples and server-side SLO queries.
	MetricsBackendURL     string
	MetricsBackendTimeout time.Duration
	SLOEvaluationInterval time.Duration
	SLOStaleAfter         time.Duration

	// Project-based API route monitoring. All values have safe defaults so
	// no environment changes are required to run the new subsystem.
	RouteSchedulerInterval time.Duration
	// RouteBroadPollingEnabled is an explicit temporary compatibility switch.
	// V2 defaults to catalog-only routes and must not probe every import.
	RouteBroadPollingEnabled bool
	RouteDueBatchSize        int
	RouteCheckRetention      time.Duration
	RouteCheckPruneBatch     int
	RouteAggregateInterval   time.Duration
	RouteAggregateWindow     time.Duration
	RouteMaxTimeout          time.Duration
	RouteMaxRedirects        int
	// RouteAllowPrivateTargets opts in to monitoring hosts on private or
	// loopback networks. It defaults to false so untrusted, user-supplied
	// URLs cannot be used to reach internal services (SSRF). Cloud metadata
	// endpoints stay blocked even when this is enabled.
	RouteAllowPrivateTargets bool
	RouteUserAgent           string
	// AgentConfigSigningKey signs short-lived, outbound-only private-agent
	// configuration envelopes. Empty disables the config endpoint safely.
	AgentConfigSigningKey []byte
}

func Load() (Config, error) {
	_ = godotenv.Load()
	cfg := Config{HTTPAddr: envOrDefault("HTTP_ADDR", ":8080"), MySQLDSN: envOrDefault("MYSQL_DSN", "argus:argus@tcp(localhost:3306)/argus?parseTime=true"), RedisAddr: envOrDefault("REDIS_ADDR", "localhost:6379"), RedisPassword: envOrDefault("REDIS_PASSWORD", ""), SchedulerInterval: mustDuration("SCHEDULER_INTERVAL", 30*time.Second), WorkerConcurrency: mustInt("WORKER_CONCURRENCY", 10), QueueCriticalWeight: mustInt("QUEUE_CRITICAL_WEIGHT", 6), QueueDefaultWeight: mustInt("QUEUE_DEFAULT_WEIGHT", 4), DueCheckBatchSize: mustInt("DUE_CHECK_BATCH_SIZE", 200), APIKey: os.Getenv("API_KEY"), DBMaxOpenConns: mustInt("DB_MAX_OPEN_CONNS", 25), DBMaxIdleConns: mustInt("DB_MAX_IDLE_CONNS", 25), DBConnMaxLifetime: mustDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute)}
	cfg.MetricsBackendURL = envOrDefault("METRICS_BACKEND_URL", "http://localhost:8428")
	cfg.MetricsBackendTimeout = mustDuration("METRICS_BACKEND_TIMEOUT", 5*time.Second)
	cfg.SLOEvaluationInterval = mustDuration("SLO_EVALUATION_INTERVAL", time.Minute)
	cfg.SLOStaleAfter = mustDuration("SLO_STALE_AFTER", 10*time.Minute)

	cfg.RouteSchedulerInterval = mustDuration("ROUTE_SCHEDULER_INTERVAL", 15*time.Second)
	cfg.RouteBroadPollingEnabled = mustBool("ROUTE_BROAD_POLLING_ENABLED", false)
	cfg.RouteDueBatchSize = mustInt("ROUTE_DUE_BATCH_SIZE", 200)
	cfg.RouteCheckRetention = mustDuration("ROUTE_CHECK_RETENTION", 30*24*time.Hour)
	cfg.RouteCheckPruneBatch = mustInt("ROUTE_CHECK_PRUNE_BATCH", 5000)
	cfg.RouteAggregateInterval = mustDuration("ROUTE_AGGREGATE_INTERVAL", 60*time.Second)
	cfg.RouteAggregateWindow = mustDuration("ROUTE_AGGREGATE_WINDOW", 24*time.Hour)
	cfg.RouteMaxTimeout = mustDuration("ROUTE_MAX_TIMEOUT", 30*time.Second)
	cfg.RouteMaxRedirects = mustInt("ROUTE_MAX_REDIRECTS", 5)
	cfg.RouteAllowPrivateTargets = mustBool("ROUTE_ALLOW_PRIVATE_TARGETS", false)
	cfg.RouteUserAgent = envOrDefault("ROUTE_USER_AGENT", "Argus-Monitor/1.0")
	cfg.AuthCookieSecure = mustBool("AUTH_COOKIE_SECURE", false)
	cfg.RecoveryDeliveryURL = os.Getenv("RECOVERY_DELIVERY_URL")
	cfg.RecoveryDeliveryTimeout = mustDuration("RECOVERY_DELIVERY_TIMEOUT", 5*time.Second)
	var keyErr error
	if cfg.RouteSecretEncryptionKey, keyErr = secrets.ParseKey(os.Getenv("ROUTE_SECRET_ENCRYPTION_KEY")); keyErr != nil {
		return Config{}, keyErr
	}
	if raw := os.Getenv("AGENT_CONFIG_SIGNING_KEY"); raw != "" {
		key, decodeErr := base64.RawURLEncoding.DecodeString(raw)
		if decodeErr != nil || len(key) != ed25519.PrivateKeySize {
			return Config{}, fmt.Errorf("AGENT_CONFIG_SIGNING_KEY must be a base64url Ed25519 private key")
		}
		cfg.AgentConfigSigningKey = key
	}

	dbIndex, err := strconv.Atoi(envOrDefault("REDIS_DB", "0"))
	if err != nil {
		return Config{}, fmt.Errorf("parse REDIS_DB: %w", err)
	}
	cfg.RedisDB = dbIndex
	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func mustInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
func mustBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
func mustDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
