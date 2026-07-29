package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"time"

	"argus/internal/adapters/outbound/mysql"
	"argus/internal/adapters/outbound/notifier"
	"argus/internal/adapters/outbound/recovery"
	"argus/internal/adapters/outbound/victoriametrics"
	"argus/internal/agent"
	"argus/internal/api"
	"argus/internal/application"
	"argus/internal/config"
	"argus/internal/observability"
	"argus/internal/platform/httpserver"
	"argus/internal/platform/storage"
	workerplatform "argus/internal/platform/worker"
	"argus/internal/worker"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	metricscollector "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracecollector "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

type Application struct {
	cfg         config.Config
	db          *sql.DB
	httpApp     *fiber.App
	workerRt    *workerplatform.Runtime
	asynqClient *asynq.Client
	logger      *observability.LogStore
	telemetry   *observability.Telemetry
	otlpGRPC    *grpc.Server
}

func New(ctx context.Context, cfg config.Config) (*Application, error) {
	logger := observability.NewLogStore(1000)
	telemetry := observability.NewTelemetry("argus", "v2")
	db, err := storage.OpenMySQL(ctx, cfg.MySQLDSN, storage.DBConfig{MaxOpenConns: cfg.DBMaxOpenConns, MaxIdleConns: cfg.DBMaxIdleConns, ConnMaxLifetime: cfg.DBConnMaxLifetime})
	if err != nil {
		_ = telemetry.Shutdown(ctx)
		return nil, err
	}
	if err = storage.ApplyMigrations(ctx, db, "db/migrations"); err != nil {
		_ = db.Close()
		_ = telemetry.Shutdown(ctx)
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	store := mysql.NewStore(db, cfg.RouteSecretEncryptionKey)
	recoveryDelivery, err := recovery.NewWebhookDelivery(cfg.RecoveryDeliveryURL, cfg.RecoveryDeliveryTimeout)
	if err != nil {
		_ = db.Close()
		_ = telemetry.Shutdown(ctx)
		return nil, fmt.Errorf("configure password recovery delivery: %w", err)
	}
	appService := application.NewService(store, store, store, store, store, store, logger, store, store, store, recoveryDelivery, store, store, store, store, store, store, store, store, store)
	appService.SetPrivateAgentStore(store)
	appService.SetPrivateAgentResultStore(store)
	appService.SetPrivateAgentAssignmentStore(store)
	if len(cfg.AgentConfigSigningKey) > 0 {
		signer, signerErr := agent.NewConfigurationSigner(cfg.AgentConfigSigningKey)
		if signerErr != nil {
			_ = db.Close()
			_ = telemetry.Shutdown(ctx)
			return nil, fmt.Errorf("configure agent configuration signing: %w", signerErr)
		}
		appService.SetAgentConfigurationSigner(signer)
	}
	appService.SetProjectIncidentStore(store)
	metricSink, err := victoriametrics.NewWriter(cfg.MetricsBackendURL, cfg.MetricsBackendTimeout)
	if err != nil {
		_ = db.Close()
		_ = telemetry.Shutdown(ctx)
		return nil, fmt.Errorf("configure metrics backend: %w", err)
	}
	metricReader, err := victoriametrics.NewReader(cfg.MetricsBackendURL, cfg.MetricsBackendTimeout)
	if err != nil {
		_ = db.Close()
		_ = telemetry.Shutdown(ctx)
		return nil, fmt.Errorf("configure metrics reader: %w", err)
	}
	telemetryIngestHandler := api.NewTelemetryIngestHandler(appService, metricSink)
	httpApp := httpserver.NewFiberAppWithMetricSinkAndTelemetryHandler(appService, logger, cfg.APIKey, metricSink, telemetryIngestHandler, cfg.AuthCookieSecure)
	var otlpGRPC *grpc.Server
	if cfg.OTLPGRPCAddr != "" {
		metricsServer, tracesServer := api.NewTelemetryGRPCServers(telemetryIngestHandler)
		otlpGRPC = grpc.NewServer(grpc.MaxRecvMsgSize(4*1024*1024), grpc.MaxSendMsgSize(1024*1024))
		metricscollector.RegisterMetricsServiceServer(otlpGRPC, metricsServer)
		tracecollector.RegisterTraceServiceServer(otlpGRPC, tracesServer)
	}
	asynqClient := asynq.NewClient(workerplatform.RedisClientOptions(cfg))
	routeEvaluator := worker.NewRouteEvaluator(worker.EvaluatorConfig{
		AllowPrivateTargets: cfg.RouteAllowPrivateTargets,
		MaxRedirects:        cfg.RouteMaxRedirects,
		MaxTimeout:          cfg.RouteMaxTimeout,
		UserAgent:           cfg.RouteUserAgent,
	})
	routeMonitorCfg := worker.RouteMonitorConfig{
		DueBatchSize:       cfg.RouteDueBatchSize,
		CheckRetention:     cfg.RouteCheckRetention,
		PruneBatchSize:     cfg.RouteCheckPruneBatch,
		AggregationWindow:  cfg.RouteAggregateWindow,
		ProjectDailyBudget: cfg.RouteProjectDailyBudget,
		GlobalDailyBudget:  cfg.RouteGlobalDailyBudget,
		ProjectConcurrency: cfg.RouteProjectConcurrency,
		GlobalConcurrency:  cfg.RouteGlobalConcurrency,
	}
	processor := worker.NewProcessor(store, store, store, appService, asynqClient, notifier.NewHTTPNotifier(), logger, store, routeEvaluator, routeMonitorCfg)
	processor.SetSLOEvaluator(worker.NewSLOEvaluator(store, metricReader, cfg.SLOStaleAfter, store))
	processor.SetAgentLivenessEvaluator(worker.NewAgentLivenessEvaluator(store, store, store))
	processor.SetHeartbeatLivenessEvaluator(worker.NewHeartbeatLivenessEvaluator(store, store))
	workerRt, err := workerplatform.NewRuntime(cfg, processor, logger)
	if err != nil {
		_ = asynqClient.Close()
		_ = db.Close()
		_ = telemetry.Shutdown(ctx)
		return nil, err
	}
	return &Application{cfg: cfg, db: db, httpApp: httpApp, workerRt: workerRt, asynqClient: asynqClient, logger: logger, telemetry: telemetry, otlpGRPC: otlpGRPC}, nil
}

func (a *Application) Start() error {
	var grpcListener net.Listener
	if a.otlpGRPC != nil {
		var grpcErr error
		grpcListener, grpcErr = net.Listen("tcp", a.cfg.OTLPGRPCAddr)
		if grpcErr != nil {
			return fmt.Errorf("listen OTLP gRPC on %s: %w", a.cfg.OTLPGRPCAddr, grpcErr)
		}
	}
	l, err := net.Listen("tcp", a.cfg.HTTPAddr)
	if err != nil {
		if grpcListener != nil {
			_ = grpcListener.Close()
		}
		return fmt.Errorf("listen on %s: %w", a.cfg.HTTPAddr, err)
	}
	go func() {
		if e := a.httpApp.Listener(l); e != nil {
			log.Printf("fiber server stopped: %v", e)
		}
	}()
	if grpcListener != nil {
		go func() {
			if e := a.otlpGRPC.Serve(grpcListener); e != nil {
				log.Printf("OTLP gRPC server stopped: %v", e)
			}
		}()
	}
	return nil
}
func (a *Application) Shutdown(ctx context.Context) error {
	shutdownErr := a.httpApp.ShutdownWithContext(ctx)
	if a.otlpGRPC != nil {
		a.otlpGRPC.Stop()
	}
	a.workerRt.Shutdown()
	_ = a.asynqClient.Close()
	_ = a.db.Close()
	_ = a.telemetry.Shutdown(ctx)
	if shutdownErr != nil {
		return fmt.Errorf("shutdown http app: %w", shutdownErr)
	}
	return nil
}
func DefaultShutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
