package notifier

import (
	"bytes"
	"context"
	"net/http"
	"time"

	"argus/internal/models"
)

type HTTPNotifier struct{ client *http.Client }

func NewHTTPNotifier() *HTTPNotifier {
	return &HTTPNotifier{client: &http.Client{Timeout: 4 * time.Second}}
}

func (n *HTTPNotifier) Notify(ctx context.Context, channels []models.AlertChannel, payload []byte) error {
	for _, channel := range channels {
		switch channel.ChannelType {
		case "webhook", "slack":
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, channel.Target, bytes.NewReader(payload))
			if err != nil {
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := n.client.Do(req)
			if err == nil && resp != nil {
				resp.Body.Close()
			}
		}
	}
	return nil
}
