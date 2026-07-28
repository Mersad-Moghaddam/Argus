// Package recovery provides the optional trusted delivery boundary for account
// recovery. It is intentionally configured by operators, never by a request.
package recovery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrDeliveryDisabled = errors.New("password recovery delivery is not configured")

type WebhookDelivery struct {
	endpoint string
	client   *http.Client
}

func NewWebhookDelivery(endpoint string, timeout time.Duration) (*WebhookDelivery, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint != "" {
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			return nil, errors.New("recovery delivery URL must be an absolute HTTPS URL without userinfo or fragment")
		}
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &WebhookDelivery{endpoint: endpoint, client: &http.Client{Timeout: timeout}}, nil
}

func (d *WebhookDelivery) DeliverPasswordRecovery(ctx context.Context, email, token string, expiresAt time.Time) error {
	if d.endpoint == "" {
		return ErrDeliveryDisabled
	}
	payload, err := json.Marshal(struct {
		Email     string    `json:"email"`
		Token     string    `json:"token"`
		ExpiresAt time.Time `json:"expiresAt"`
	}{Email: email, Token: token, ExpiresAt: expiresAt})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("recovery delivery rejected the request")
	}
	return nil
}
