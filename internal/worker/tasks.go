package worker

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const (
	TypeEnqueueDueChecks = "website:enqueue_due_checks"
	TypeCheckWebsite     = "website:check"
	TypeDispatchOutbox   = "outbox:dispatch"
)

type CheckWebsitePayload struct {
	WebsiteID      int64   `json:"websiteId"`
	URL            string  `json:"url"`
	HealthCheckURL *string `json:"healthCheckUrl,omitempty"`
	Interval       int     `json:"interval"`
}

func NewEnqueueDueChecksTask() *asynq.Task { return asynq.NewTask(TypeEnqueueDueChecks, nil) }
func NewDispatchOutboxTask() *asynq.Task   { return asynq.NewTask(TypeDispatchOutbox, nil) }

func NewCheckWebsiteTask(payload CheckWebsitePayload) (*asynq.Task, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal check website payload: %w", err)
	}
	return asynq.NewTask(TypeCheckWebsite, body), nil
}
