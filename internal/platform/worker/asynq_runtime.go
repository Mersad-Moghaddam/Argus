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

type Runtime struct {
	Server    *asynq.Server
	Scheduler *asynq.Scheduler
}

func NewRuntime(cfg config.Config, processor *appworker.Processor, logger *observability.LogStore) (*Runtime, error) {
	redisOptions := RedisClientOptions(cfg)
	if err := verifyRedisConnection(redisOptions); err != nil {
		return nil, err
	}
	server := asynq.NewServer(redisOptions, asynq.Config{Concurrency: cfg.WorkerConcurrency, Queues: map[string]int{"critical": cfg.QueueCriticalWeight, "default": cfg.QueueDefaultWeight}})
	scheduler := asynq.NewScheduler(redisOptions, &asynq.SchedulerOpts{})
	if _, err := scheduler.Register(fmt.Sprintf("@every %s", cfg.SchedulerInterval), appworker.NewEnqueueDueChecksTask(), asynq.Queue("default")); err != nil {
		return nil, err
	}
	if _, err := scheduler.Register("@every 10s", appworker.NewDispatchOutboxTask(), asynq.Queue("default")); err != nil {
		return nil, err
	}
	mux := asynq.NewServeMux()
	processor.Register(mux)
	go func() { _ = server.Run(mux) }()
	go func() { _ = scheduler.Run() }()
	logger.Add("info", "system", "worker_runtime_started", "Worker runtime started", nil, nil)
	return &Runtime{Server: server, Scheduler: scheduler}, nil
}
func (r *Runtime) Shutdown() { r.Scheduler.Shutdown(); r.Server.Shutdown() }
func RedisClientOptions(cfg config.Config) asynq.RedisClientOpt {
	return asynq.RedisClientOpt{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB}
}
func verifyRedisConnection(options asynq.RedisClientOpt) error {
	c := redis.NewClient(&redis.Options{Addr: options.Addr, Password: options.Password, DB: options.DB})
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return c.Ping(ctx).Err()
}
