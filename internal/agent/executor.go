package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxAssignmentTimeout = 60 * time.Second

// ExecuteAssignment performs only the narrow HTTP operation permitted by a
// signed assignment. It never follows a redirect, sends credentials, or keeps
// an unbounded response body in memory.
func ExecuteAssignment(ctx context.Context, a Assignment, client *http.Client) (bool, string) {
	if a.ID <= 0 || (a.Method != http.MethodGet && a.Method != http.MethodHead) || a.TimeoutMS < 200 || a.TimeoutMS > int(maxAssignmentTimeout.Milliseconds()) {
		return false, "invalid assignment"
	}
	u, err := url.Parse(strings.TrimSpace(a.Target))
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.Fragment != "" {
		return false, "invalid assignment target"
	}
	timeout := time.Duration(a.TimeoutMS) * time.Millisecond
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, a.Method, u.String(), nil)
	if err != nil {
		return false, "invalid assignment request"
	}
	if client == nil {
		client = &http.Client{}
	}
	copy := *client
	copy.Timeout = timeout
	copy.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := copy.Do(req)
	if err != nil {
		return false, "request failed"
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return false, fmt.Sprintf("unexpected HTTP status %d", resp.StatusCode)
	}
	return true, ""
}
