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
	"argus/internal/adapters/outbound/victoriametrics"
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
)

type Application struct {
	cfg         config.Config
	db          *sql.DB
	httpApp     *fiber.App
	workerRt    *workerplatform.Runtime
	asynqClient *asynq.Client
	logger      *observability.LogStore
	telemetry   *observability.Telemetry
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
	store := mysql.NewStore(db)
	appService := application.NewService(store, store, store, store, store, store, logger, store, store, store, store, store, store, store, store, store, store)
	metricSink, err := victoriametrics.NewWriter(cfg.MetricsBackendURL, cfg.MetricsBackendTimeout)
	if err != nil {
		_ = db.Close()
		_ = telemetry.Shutdown(ctx)
		return nil, fmt.Errorf("configure metrics backend: %w", err)
	}
	httpApp := httpserver.NewFiberAppWithMetricSink(appService, logger, cfg.APIKey, metricSink, cfg.AuthCookieSecure)
	asynqClient := asynq.NewClient(workerplatform.RedisClientOptions(cfg))
	routeEvaluator := worker.NewRouteEvaluator(worker.EvaluatorConfig{
		AllowPrivateTargets: cfg.RouteAllowPrivateTargets,
		MaxRedirects:        cfg.RouteMaxRedirects,
		MaxTimeout:          cfg.RouteMaxTimeout,
		UserAgent:           cfg.RouteUserAgent,
	})
	routeMonitorCfg := worker.RouteMonitorConfig{
		DueBatchSize:      cfg.RouteDueBatchSize,
		CheckRetention:    cfg.RouteCheckRetention,
		PruneBatchSize:    cfg.RouteCheckPruneBatch,
		AggregationWindow: cfg.RouteAggregateWindow,
	}
	processor := worker.NewProcessor(store, store, store, appService, asynqClient, notifier.NewHTTPNotifier(), logger, store, routeEvaluator, routeMonitorCfg)
	workerRt, err := workerplatform.NewRuntime(cfg, processor, logger)
	if err != nil {
		_ = asynqClient.Close()
		_ = db.Close()
		_ = telemetry.Shutdown(ctx)
		return nil, err
	}
	return &Application{cfg: cfg, db: db, httpApp: httpApp, workerRt: workerRt, asynqClient: asynqClient, logger: logger, telemetry: telemetry}, nil
}

func (a *Application) Start() error {
	l, err := net.Listen("tcp", a.cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", a.cfg.HTTPAddr, err)
	}
	go func() {
		if e := a.httpApp.Listener(l); e != nil {
			log.Printf("fiber server stopped: %v", e)
		}
	}()
	return nil
}
func (a *Application) Shutdown(ctx context.Context) error {
	shutdownErr := a.httpApp.ShutdownWithContext(ctx)
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
