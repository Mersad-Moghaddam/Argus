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
	return &HTTPNotifier{client: &http.Client{
		Timeout: 4 * time.Second,
		// Notification destinations are configured explicitly. Do not follow a
		// redirect that could move an incident payload to another origin.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (n *HTTPNotifier) Notify(ctx context.Context, channels []models.AlertChannel, payload []byte) error {
	var firstErr error
	for _, channel := range channels {
		if !channel.Enabled {
			continue
		}
		switch channel.ChannelType {
		case "webhook", "slack":
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, channel.Target, bytes.NewReader(payload))
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := n.client.Do(req)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				continue
			}
			if resp != nil {
				resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					if firstErr == nil {
						firstErr = fmt.Errorf("notification endpoint returned HTTP %d", resp.StatusCode)
					}
				}
			}
		}
	}
	return firstErr
}
