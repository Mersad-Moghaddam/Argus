package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"time"

	"argus/internal/config"
	"argus/internal/observability"
	"argus/internal/platform/httpserver"
	"argus/internal/platform/storage"
	workerplatform "argus/internal/platform/worker"
	"argus/internal/repository"
	"argus/internal/service"
	"argus/internal/worker"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
)

// Application encapsulates all runtime dependencies.
type Application struct {
	cfg         config.Config
	db          *sql.DB
	httpApp     *fiber.App
	workerRt    *workerplatform.Runtime
	asynqClient *asynq.Client
	logger      *observability.LogStore
}

// New creates and wires application dependencies.
func New(ctx context.Context, cfg config.Config) (*Application, error) {
	logger := observability.NewLogStore(1000)
	logger.Add("info", "system", "bootstrap_started", "Application bootstrap started", nil, map[string]string{"httpAddr": cfg.HTTPAddr})

	db, err := storage.OpenMySQL(ctx, cfg.MySQLDSN)
	if err != nil {
		logger.Add("error", "system", "mysql_connection_failed", "Failed to connect to MySQL", nil, map[string]string{"error": err.Error()})
		return nil, err
	}
	logger.Add("info", "system", "mysql_connected", "MySQL connection established", nil, nil)

	repo := repository.NewMySQLWebsiteRepository(db)
	websiteService := service.NewWebsiteService(repo, logger)
	httpApp := httpserver.NewFiberApp(websiteService, logger)

	asynqClient := asynq.NewClient(workerplatform.RedisClientOptions(cfg))
	processor := worker.NewProcessor(repo, asynqClient, logger)
	workerRt, err := workerplatform.NewRuntime(cfg, processor, logger)
	if err != nil {
		_ = asynqClient.Close()
		_ = db.Close()
		return nil, err
	}

	logger.Add("info", "system", "bootstrap_completed", "Application bootstrap completed", nil, nil)

	return &Application{
		cfg:         cfg,
		db:          db,
		httpApp:     httpApp,
		workerRt:    workerRt,
		asynqClient: asynqClient,
		logger:      logger,
	}, nil
}

// Start starts HTTP server.
func (a *Application) Start() error {
	listener, err := net.Listen("tcp", a.cfg.HTTPAddr)
	if err != nil {
		a.logger.Add("error", "system", "http_bind_failed", "Failed to bind HTTP listener", nil, map[string]string{"error": err.Error(), "httpAddr": a.cfg.HTTPAddr})
		return fmt.Errorf("listen on %s: %w", a.cfg.HTTPAddr, err)
	}
	a.logger.Add("info", "system", "http_listener_started", "HTTP listener started", nil, map[string]string{"httpAddr": a.cfg.HTTPAddr})

	go func() {
		if serveErr := a.httpApp.Listener(listener); serveErr != nil {
			a.logger.Add("error", "system", "http_server_stopped", "Fiber HTTP server stopped", nil, map[string]string{"error": serveErr.Error()})
			log.Printf("fiber server stopped: %v", serveErr)
		}
	}()

	return nil
}

// Shutdown gracefully stops all components.
func (a *Application) Shutdown(ctx context.Context) error {
	a.logger.Add("info", "system", "shutdown_started", "Application shutdown started", nil, nil)

	shutdownErr := a.httpApp.ShutdownWithContext(ctx)
	if shutdownErr != nil {
		a.logger.Add("error", "system", "http_shutdown_failed", "Failed to shutdown HTTP server", nil, map[string]string{"error": shutdownErr.Error()})
		log.Printf("shutdown fiber: %v", shutdownErr)
	}

	a.workerRt.Shutdown()
	a.logger.Add("info", "system", "worker_shutdown_completed", "Asynq worker and scheduler shutdown completed", nil, nil)

	if err := a.asynqClient.Close(); err != nil {
		a.logger.Add("error", "system", "asynq_client_close_failed", "Failed to close Asynq client", nil, map[string]string{"error": err.Error()})
		log.Printf("shutdown asynq client: %v", err)
	}

	if err := a.db.Close(); err != nil {
		a.logger.Add("error", "system", "mysql_close_failed", "Failed to close MySQL connection", nil, map[string]string{"error": err.Error()})
		log.Printf("close mysql: %v", err)
	}

	a.logger.Add("info", "system", "shutdown_completed", "Application shutdown completed", nil, nil)

	if shutdownErr != nil {
		return fmt.Errorf("shutdown http app: %w", shutdownErr)
	}
	return nil
}

// DefaultShutdownContext returns common shutdown context.
func DefaultShutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
