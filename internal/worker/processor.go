package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"argus/internal/observability"
	"argus/internal/repository"
	"github.com/hibiken/asynq"
)

// Processor handles Asynq jobs.
type Processor struct {
	repo   repository.WebsiteRepository
	client *asynq.Client
	logger *observability.LogStore
}

// NewProcessor creates Processor.
func NewProcessor(repo repository.WebsiteRepository, client *asynq.Client, logger *observability.LogStore) *Processor {
	return &Processor{repo: repo, client: client, logger: logger}
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
		p.logger.Add("error", "worker", "fetch_due_websites_failed", "Failed to query due websites", nil, map[string]string{"error": err.Error()})
		return fmt.Errorf("list due websites: %w", err)
	}

	for _, website := range dueWebsites {
		task, taskErr := NewCheckWebsiteTask(CheckWebsitePayload{
			WebsiteID:      website.ID,
			URL:            website.URL,
			HealthCheckURL: website.HealthCheckURL,
			Interval:       website.CheckInterval,
		})
		if taskErr != nil {
			p.logger.Add("error", "worker", "build_check_task_failed", "Failed to build check task payload", &website.ID, map[string]string{"error": taskErr.Error()})
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
			p.logger.Add("error", "worker", "enqueue_check_task_failed", "Failed to enqueue website check task", &website.ID, map[string]string{"error": enqueueErr.Error()})
			return fmt.Errorf("enqueue website check task: %w", enqueueErr)
		}

		checkTarget := website.URL
		if website.HealthCheckURL != nil {
			checkTarget = *website.HealthCheckURL
		}
		p.logger.Add("info", "worker", "check_task_enqueued", "Website check task enqueued", &website.ID, map[string]string{
			"url":           website.URL,
			"checkTarget":   checkTarget,
			"checkInterval": strconv.Itoa(website.CheckInterval),
			"scheduledFor":  website.NextCheckAt.Format(time.RFC3339),
		})
	}
	return nil
}

// HandleCheckWebsite checks website and updates state in MySQL.
func (p *Processor) HandleCheckWebsite(ctx context.Context, task *asynq.Task) error {
	var payload CheckWebsitePayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		p.logger.Add("error", "worker", "parse_check_payload_failed", "Failed to parse check task payload", nil, map[string]string{"error": err.Error()})
		return fmt.Errorf("unmarshal check website task payload: %w", err)
	}

	checkURL := payload.URL
	if payload.HealthCheckURL != nil {
		checkURL = *payload.HealthCheckURL
	}

	status := "down"
	statusCode := 0
	latencyMS := 0
	failureReason := ""

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, checkURL, nil)
	if err != nil {
		p.logger.Add("error", "worker", "build_http_request_failed", "Failed to build HTTP request for website check", &payload.WebsiteID, map[string]string{"url": checkURL, "error": err.Error()})
		return fmt.Errorf("build check request: %w", err)
	}

	start := time.Now()
	response, err := http.DefaultClient.Do(req)
	latencyMS = int(time.Since(start).Milliseconds())
	if err == nil {
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, response.Body)
		statusCode = response.StatusCode
		if response.StatusCode >= 200 && response.StatusCode < 400 {
			status = "up"
		} else {
			failureReason = "non-successful status code"
		}
	} else {
		failureReason = err.Error()
	}

	checkedAt := time.Now().UTC()
	nextCheckAt := checkedAt.Add(time.Duration(payload.Interval) * time.Second)
	err = p.repo.MarkChecked(ctx, payload.WebsiteID, status, statusCode, latencyMS, checkedAt, nextCheckAt)
	if err != nil {
		p.logger.Add("error", "worker", "mark_checked_failed", "Failed to persist website check result", &payload.WebsiteID, map[string]string{"error": err.Error()})
		return fmt.Errorf("mark website checked: %w", err)
	}

	details := map[string]string{
		"url":           payload.URL,
		"checkTarget":   checkURL,
		"status":        status,
		"statusCode":    strconv.Itoa(statusCode),
		"latencyMs":     strconv.Itoa(latencyMS),
		"checkedAt":     checkedAt.Format(time.RFC3339),
		"nextCheckAt":   nextCheckAt.Format(time.RFC3339),
		"checkInterval": strconv.Itoa(payload.Interval),
	}
	if failureReason != "" {
		details["failureReason"] = failureReason
	}

	level := "info"
	message := "Website health check completed"
	if status == "down" {
		level = "warn"
		message = "Website health check reported downtime"
	}
	p.logger.Add(level, "worker", "website_checked", message, &payload.WebsiteID, details)

	return nil
}
