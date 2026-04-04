package notifier

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"argus/internal/models"
)

type HTTPNotifier struct{ client *http.Client }

func NewHTTPNotifier() *HTTPNotifier {
	return &HTTPNotifier{client: &http.Client{Timeout: 4 * time.Second}}
}

func (n *HTTPNotifier) Notify(ctx context.Context, channels []models.AlertChannel, payload []byte) error {
	failures := make([]string, 0)
	for _, channel := range channels {
		switch channel.ChannelType {
		case "webhook", "slack":
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, channel.Target, bytes.NewReader(payload))
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", channel.Name, err))
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := n.client.Do(req)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", channel.Name, err))
				continue
			}
			if resp != nil {
				resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					failures = append(failures, fmt.Sprintf("%s: http %d", channel.Name, resp.StatusCode))
				}
			}
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("notifier delivery failures: %v", failures)
	}
	return nil
}
