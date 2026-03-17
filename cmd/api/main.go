package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"argus/internal/api"
	"argus/internal/config"
	"argus/internal/repository"
	"argus/internal/service"
	"argus/internal/worker"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("open mysql connection: %v", err)
	}
	defer db.Close()

	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err = db.PingContext(pingCtx); err != nil {
		pingCancel()
		log.Fatalf("ping mysql: %v", err)
	}
	pingCancel()

	redisOptions := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	repo := repository.NewMySQLWebsiteRepository(db)
	websiteService := service.NewWebsiteService(repo)

	asynqClient := asynq.NewClient(redisOptions)
	defer asynqClient.Close()

	processor := worker.NewProcessor(repo, asynqClient)
	server := asynq.NewServer(redisOptions, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 6,
			"default":  4,
		},
	})

	mux := asynq.NewServeMux()
	processor.Register(mux)

	go func() {
		if runErr := server.Run(mux); runErr != nil {
			log.Printf("asynq server stopped: %v", runErr)
		}
	}()

	scheduler := asynq.NewScheduler(redisOptions, &asynq.SchedulerOpts{})
	spec := fmt.Sprintf("@every %s", cfg.SchedulerInterval)
	if _, err = scheduler.Register(spec, worker.NewEnqueueDueChecksTask(), asynq.Queue("default")); err != nil {
		log.Fatalf("register scheduler task: %v", err)
	}

	go func() {
		if runErr := scheduler.Run(); runErr != nil {
			log.Printf("asynq scheduler stopped: %v", runErr)
		}
	}()

	app := fiber.New(fiber.Config{AppName: "Argus Distributed Uptime Checker"})
	handler := api.NewWebsiteHandler(websiteService)
	apiGroup := app.Group("/api")
	api.RegisterWebsiteRoutes(apiGroup, handler)
	app.Static("/", "./web")

	go func() {
		if listenErr := app.Listen(cfg.HTTPAddr); listenErr != nil {
			log.Printf("fiber server stopped: %v", listenErr)
		}
	}()

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	<-shutdownSignal

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err = app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("shutdown fiber: %v", err)
	}

	scheduler.Shutdown()
	server.Shutdown()

	if err = db.Close(); err != nil {
		log.Printf("close mysql: %v", err)
	}
}
