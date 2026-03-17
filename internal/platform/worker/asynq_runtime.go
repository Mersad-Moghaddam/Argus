package worker

import (
	"context"
	"fmt"
	"time"

	"argus/internal/config"
	"argus/internal/observability"
	appworker "argus/internal/worker"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
)

// Runtime contains Asynq server and scheduler.
type Runtime struct {
	Server    *asynq.Server
	Scheduler *asynq.Scheduler
}

// NewRuntime configures Asynq server and scheduler.
func NewRuntime(cfg config.Config, processor *appworker.Processor, logger *observability.LogStore) (*Runtime, error) {
	redisOptions := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

	if err := verifyRedisConnection(redisOptions); err != nil {
		logger.Add("error", "system", "redis_connectivity_check_failed", "Unable to connect to Redis during startup", nil, map[string]string{"error": err.Error()})
		return nil, fmt.Errorf("verify redis connection: %w", err)
	}
	logger.Add("info", "system", "redis_connectivity_check_passed", "Redis connectivity validated", nil, map[string]string{"redisAddress": cfg.RedisAddr})

	server := asynq.NewServer(redisOptions, asynq.Config{
		Concurrency: 10,
		Queues: map[string]int{
			"critical": 6,
			"default":  4,
		},
	})

	scheduler := asynq.NewScheduler(redisOptions, &asynq.SchedulerOpts{})
	spec := fmt.Sprintf("@every %s", cfg.SchedulerInterval)
	if _, err := scheduler.Register(spec, appworker.NewEnqueueDueChecksTask(), asynq.Queue("default")); err != nil {
		logger.Add("error", "system", "scheduler_registration_failed", "Unable to register scheduler task", nil, map[string]string{"error": err.Error(), "spec": spec})
		return nil, fmt.Errorf("register scheduler task: %w", err)
	}

	mux := asynq.NewServeMux()
	processor.Register(mux)

	go func() {
		logger.Add("info", "system", "worker_server_started", "Asynq worker server started", nil, nil)
		if runErr := server.Run(mux); runErr != nil {
			logger.Add("error", "system", "worker_server_stopped", "Asynq worker server stopped unexpectedly", nil, map[string]string{"error": runErr.Error()})
		}
	}()

	go func() {
		logger.Add("info", "system", "scheduler_started", "Asynq scheduler started", nil, map[string]string{"spec": spec})
		if runErr := scheduler.Run(); runErr != nil {
			logger.Add("error", "system", "scheduler_stopped", "Asynq scheduler stopped unexpectedly", nil, map[string]string{"error": runErr.Error()})
		}
	}()

	return &Runtime{Server: server, Scheduler: scheduler}, nil
}

// Shutdown gracefully shuts down worker resources.
func (r *Runtime) Shutdown() {
	r.Scheduler.Shutdown()
	r.Server.Shutdown()
}

// RedisClientOptions builds options for creating Asynq clients.
func RedisClientOptions(cfg config.Config) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}
}

func verifyRedisConnection(options asynq.RedisClientOpt) error {
	client := redis.NewClient(&redis.Options{
		Addr:     options.Addr,
		Password: options.Password,
		DB:       options.DB,
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return err
	}
	return nil
}
