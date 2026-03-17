package worker

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const (
	// TypeEnqueueDueChecks fetches due websites and schedules check tasks.
	TypeEnqueueDueChecks = "website:enqueue_due_checks"
	// TypeCheckWebsite performs a single website uptime check.
	TypeCheckWebsite = "website:check"
)

// CheckWebsitePayload is payload for TypeCheckWebsite.
type CheckWebsitePayload struct {
	WebsiteID      int64   `json:"websiteId"`
	URL            string  `json:"url"`
	HealthCheckURL *string `json:"healthCheckUrl,omitempty"`
	Interval       int     `json:"interval"`
}

// NewEnqueueDueChecksTask creates a dispatcher task.
func NewEnqueueDueChecksTask() *asynq.Task {
	return asynq.NewTask(TypeEnqueueDueChecks, nil)
}

// NewCheckWebsiteTask creates a website check task.
func NewCheckWebsiteTask(payload CheckWebsitePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal check website payload: %w", err)
	}
	return asynq.NewTask(TypeCheckWebsite, body), nil
}
