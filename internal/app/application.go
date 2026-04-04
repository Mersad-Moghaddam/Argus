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
}

func New(ctx context.Context, cfg config.Config) (*Application, error) {
	logger := observability.NewLogStore(1000)
	db, err := storage.OpenMySQL(ctx, cfg.MySQLDSN, storage.DBConfig{MaxOpenConns: cfg.DBMaxOpenConns, MaxIdleConns: cfg.DBMaxIdleConns, ConnMaxLifetime: cfg.DBConnMaxLifetime})
	if err != nil {
		return nil, err
	}
	store := mysql.NewStore(db)
	appService := application.NewService(store, store, store, store, store, store, logger)
	httpApp := httpserver.NewFiberApp(appService, logger, cfg.APIKey)
	asynqClient := asynq.NewClient(workerplatform.RedisClientOptions(cfg))
	processor := worker.NewProcessor(store, store, store, appService, asynqClient, notifier.NewHTTPNotifier(), logger)
	workerRt, err := workerplatform.NewRuntime(cfg, processor, logger)
	if err != nil {
		_ = asynqClient.Close()
		_ = db.Close()
		return nil, err
	}
	return &Application{cfg: cfg, db: db, httpApp: httpApp, workerRt: workerRt, asynqClient: asynqClient, logger: logger}, nil
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
	if shutdownErr != nil {
		return fmt.Errorf("shutdown http app: %w", shutdownErr)
	}
	return nil
}
func DefaultShutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
