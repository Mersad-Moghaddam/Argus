package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"argus/internal/repository"
	"github.com/hibiken/asynq"
)

// Processor handles Asynq jobs.
type Processor struct {
	repo   repository.WebsiteRepository
	client *asynq.Client
}

// NewProcessor creates Processor.
func NewProcessor(repo repository.WebsiteRepository, client *asynq.Client) *Processor {
	return &Processor{repo: repo, client: client}
}

// Register registers task handlers.
func (p *Processor) Register(mux *asynq.ServeMux) {
	mux.HandleFunc(TypeEnqueueDueChecks, p.HandleEnqueueDueChecks)
	mux.HandleFunc(TypeCheckWebsite, p.HandleCheckWebsite)
}

// HandleEnqueueDueChecks enqueues one check task per due website.
func (p *Processor) HandleEnqueueDueChecks(ctx context.Context, _ *asynq.Task) error {
	dueWebsites, err := p.repo.ListDue(ctx, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("list due websites: %w", err)
	}

	for _, website := range dueWebsites {
		task, taskErr := NewCheckWebsiteTask(CheckWebsitePayload{
			WebsiteID: website.ID,
			URL:       website.URL,
			Interval:  website.CheckInterval,
		})
		if taskErr != nil {
			return taskErr
		}

		_, enqueueErr := p.client.EnqueueContext(
			ctx,
			task,
			asynq.Queue("critical"),
			asynq.TaskID(fmt.Sprintf("website-check-%d-%d", website.ID, website.NextCheckAt.Unix())),
		)
		if enqueueErr != nil {
			if enqueueErr == asynq.ErrTaskIDConflict {
				continue
			}
			return fmt.Errorf("enqueue website check task: %w", enqueueErr)
		}
	}
	return nil
}

// HandleCheckWebsite checks website and updates state in MySQL.
func (p *Processor) HandleCheckWebsite(ctx context.Context, task *asynq.Task) error {
	var payload CheckWebsitePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal check website task payload: %w", err)
	}

	status := "down"
	statusCode := 0

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, payload.URL, nil)
	if err != nil {
		return fmt.Errorf("build check request: %w", err)
	}

	response, err := http.DefaultClient.Do(req)
	if err == nil {
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		statusCode = response.StatusCode
		if response.StatusCode >= 200 && response.StatusCode < 400 {
			status = "up"
		}
	}

	checkedAt := time.Now().UTC()
	nextCheckAt := checkedAt.Add(time.Duration(payload.Interval) * time.Second)
	err = p.repo.MarkChecked(ctx, payload.WebsiteID, status, statusCode, checkedAt, nextCheckAt)
	if err != nil {
		return fmt.Errorf("mark website checked: %w", err)
	}
	return nil
}
