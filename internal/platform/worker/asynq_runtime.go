package worker

import (
	"fmt"

	"argus/internal/config"
	appworker "argus/internal/worker"
	"github.com/hibiken/asynq"
)

// Runtime contains Asynq server and scheduler.
type Runtime struct {
	Server    *asynq.Server
	Scheduler *asynq.Scheduler
}

// NewRuntime configures Asynq server and scheduler.
func NewRuntime(cfg config.Config, processor *appworker.Processor) (*Runtime, error) {
	redisOptions := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	}

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
		return nil, fmt.Errorf("register scheduler task: %w", err)
	}

	mux := asynq.NewServeMux()
	processor.Register(mux)

	go func() {
		if runErr := server.Run(mux); runErr != nil {
			fmt.Printf("asynq server stopped: %v\n", runErr)
		}
	}()

	go func() {
		if runErr := scheduler.Run(); runErr != nil {
			fmt.Printf("asynq scheduler stopped: %v\n", runErr)
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
