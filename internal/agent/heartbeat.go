// Package agent implements the outbound-only private-agent control-plane client.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const heartbeatPath = "/agent/heartbeat"

type Config struct {
	ControlURL string
	Token      string
	Version    string
	HTTPClient *http.Client
}

// Client owns no listener and only sends an authenticated HTTPS request to
// Argus. It never receives target addresses, work, or reverse-connect traffic.
type Client struct {
	endpoint string
	token    string
	version  string
	http     *http.Client
}

func NewClient(config Config) (*Client, error) {
	base, err := url.Parse(strings.TrimSpace(config.ControlURL))
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, errors.New("control URL must be an absolute URL")
	}
	if base.Scheme != "https" && !(base.Scheme == "http" && isLoopbackHost(base.Hostname())) {
		return nil, errors.New("control URL must use HTTPS (HTTP is allowed only for loopback development)")
	}
	token := strings.TrimSpace(config.Token)
	if !strings.HasPrefix(token, "argus_agent_") || len(token) < len("argus_agent_")+16 {
		return nil, errors.New("agent token is invalid")
	}
	base.Path = strings.TrimRight(base.Path, "/") + heartbeatPath
	base.RawQuery = ""
	base.Fragment = ""
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{endpoint: base.String(), token: token, version: strings.TrimSpace(config.Version), http: client}, nil
}

func (c *Client) Heartbeat(ctx context.Context) error {
	body, err := json.Marshal(struct {
		Version string `json:"version"`
	}{Version: c.version})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send agent heartbeat: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("agent heartbeat rejected with HTTP %d", resp.StatusCode)
	}
	return nil
}

func isLoopbackHost(host string) bool {
	host = strings.ToLower(host)
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
