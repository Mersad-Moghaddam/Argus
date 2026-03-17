package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"argus/internal/config"
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
}

// New creates and wires application dependencies.
func New(ctx context.Context, cfg config.Config) (*Application, error) {
	db, err := storage.OpenMySQL(ctx, cfg.MySQLDSN)
	if err != nil {
		return nil, err
	}

	repo := repository.NewMySQLWebsiteRepository(db)
	websiteService := service.NewWebsiteService(repo)
	httpApp := httpserver.NewFiberApp(websiteService)

	asynqClient := asynq.NewClient(workerplatform.RedisClientOptions(cfg))
	processor := worker.NewProcessor(repo, asynqClient)
	workerRt, err := workerplatform.NewRuntime(cfg, processor)
	if err != nil {
		_ = asynqClient.Close()
		_ = db.Close()
		return nil, err
	}

	return &Application{
		cfg:         cfg,
		db:          db,
		httpApp:     httpApp,
		workerRt:    workerRt,
		asynqClient: asynqClient,
	}, nil
}

// Start starts HTTP server.
func (a *Application) Start() {
	go func() {
		if err := a.httpApp.Listen(a.cfg.HTTPAddr); err != nil {
			log.Printf("fiber server stopped: %v", err)
		}
	}()
}

// Shutdown gracefully stops all components.
func (a *Application) Shutdown(ctx context.Context) error {
	shutdownErr := a.httpApp.ShutdownWithContext(ctx)
	if shutdownErr != nil {
		log.Printf("shutdown fiber: %v", shutdownErr)
	}

	a.workerRt.Shutdown()

	if err := a.asynqClient.Close(); err != nil {
		log.Printf("shutdown asynq client: %v", err)
	}

	if err := a.db.Close(); err != nil {
		log.Printf("close mysql: %v", err)
	}

	if shutdownErr != nil {
		return fmt.Errorf("shutdown http app: %w", shutdownErr)
	}
	return nil
}

// DefaultShutdownContext returns common shutdown context.
func DefaultShutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
