package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"argus/internal/models"
)

const (
	maxRouteResponseBytes = 1024 * 1024
	maxRouteRedirects     = 5
	maxRetryBackoff       = 2 * time.Second
)

var pathParameterPattern = regexp.MustCompile(`\{([^{}]+)\}`)

type RouteEvaluation struct {
	Status        string
	StatusCode    int
	LatencyMS     int
	FailureReason string
	Attempts      int
}

type targetValidator func(context.Context, string) error

type RouteEvaluator struct {
	validate targetValidator
	client   *http.Client
}

func NewRouteEvaluator() *RouteEvaluator {
	e := &RouteEvaluator{validate: validateTargetContext}
	e.client = &http.Client{
		Transport: safeTransport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRouteRedirects {
				return errors.New("too many redirects")
			}
			return e.validate(req.Context(), req.URL.String())
		},
	}
	return e
}

func safeTransport() *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("invalid target address: %w", err)
			}
			ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
			if err != nil {
				return nil, fmt.Errorf("resolve target: %w", err)
			}
			for _, ip := range ips {
				if err := validateIP(ip.Unmap()); err != nil {
					continue
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			}
			return nil, errors.New("target has no permitted public address")
		},
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   2,
		DisableCompression:    true,
	}
}

func (e *RouteEvaluator) Evaluate(ctx context.Context, route models.APIRoute, timeoutCeiling time.Duration) RouteEvaluation {
	target, err := buildRouteTarget(route)
	if err != nil {
		return RouteEvaluation{Status: "down", FailureReason: err.Error(), Attempts: 1}
	}
	if err = e.validate(ctx, target); err != nil {
		return RouteEvaluation{Status: "down", FailureReason: err.Error(), Attempts: 1}
	}
	timeout := time.Duration(route.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if timeoutCeiling > 0 && timeout > timeoutCeiling {
		timeout = timeoutCeiling
	}
	retries := route.Retries
	if retries < 0 {
		retries = 0
	}
	if retries > 5 {
		retries = 5
	}

	var result RouteEvaluation
	for attempt := 1; attempt <= retries+1; attempt++ {
		result = e.evaluateOnce(ctx, route, target, timeout)
		result.Attempts = attempt
		if result.Status == "up" || attempt > retries {
			return result
		}
		backoff := time.Duration(attempt) * 200 * time.Millisecond
		if backoff > maxRetryBackoff {
			backoff = maxRetryBackoff
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			result.FailureReason = ctx.Err().Error()
			return result
		case <-timer.C:
		}
	}
	return result
}

func (e *RouteEvaluator) evaluateOnce(ctx context.Context, route models.APIRoute, target string, timeout time.Duration) RouteEvaluation {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, route.Method, target, nil)
	if err != nil {
		return RouteEvaluation{Status: "down", FailureReason: "build request: " + err.Error()}
	}
	if err = applyRouteHeaders(req, route.Headers); err != nil {
		return RouteEvaluation{Status: "down", FailureReason: err.Error()}
	}
	started := time.Now()
	resp, err := e.client.Do(req)
	latency := int(time.Since(started).Milliseconds())
	if err != nil {
		return RouteEvaluation{Status: "down", LatencyMS: latency, FailureReason: sanitizeRequestError(err)}
	}
	defer resp.Body.Close()
	_, copyErr := io.Copy(io.Discard, io.LimitReader(resp.Body, maxRouteResponseBytes+1))
	if copyErr != nil {
		return RouteEvaluation{Status: "down", StatusCode: resp.StatusCode, LatencyMS: latency, FailureReason: "read response: " + copyErr.Error()}
	}
	if statusInRange(resp.StatusCode, route.ExpectedStatusRange) {
		return RouteEvaluation{Status: "up", StatusCode: resp.StatusCode, LatencyMS: latency}
	}
	return RouteEvaluation{Status: "down", StatusCode: resp.StatusCode, LatencyMS: latency, FailureReason: fmt.Sprintf("unexpected status code %d", resp.StatusCode)}
}

func buildRouteTarget(route models.APIRoute) (string, error) {
	if strings.TrimSpace(route.BaseURL) == "" {
		return "", errors.New("missing route base URL")
	}
	path := pathParameterPattern.ReplaceAllStringFunc(route.Path, func(match string) string {
		name := strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
		return url.PathEscape(parameterValue(route.Parameters, name))
	})
	base, err := url.Parse(strings.TrimRight(route.BaseURL, "/"))
	if err != nil {
		return "", errors.New("invalid route base URL")
	}
	relative, err := url.Parse(path)
	if err != nil {
		return "", errors.New("invalid route path")
	}
	return base.ResolveReference(relative).String(), nil
}

func parameterValue(raw, name string) string {
	var parameters []struct {
		Name    string `json:"name"`
		In      string `json:"in"`
		Example string `json:"example"`
		Default string `json:"default"`
	}
	if json.Unmarshal([]byte(raw), &parameters) == nil {
		for _, parameter := range parameters {
			if parameter.In == "path" && parameter.Name == name {
				if parameter.Example != "" {
					return parameter.Example
				}
				if parameter.Default != "" {
					return parameter.Default
				}
			}
		}
	}
	return "1"
}

func applyRouteHeaders(req *http.Request, raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return errors.New("invalid route headers")
	}
	for name, value := range headers {
		if strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return errors.New("invalid route header")
		}
		req.Header.Set(name, value)
	}
	return nil
}

func statusInRange(code int, raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		raw = "200-399"
	}
	for _, part := range strings.Split(raw, ",") {
		bounds := strings.SplitN(strings.TrimSpace(part), "-", 2)
		low, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
		if err != nil {
			continue
		}
		high := low
		if len(bounds) == 2 {
			high, err = strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err != nil {
				continue
			}
		}
		if code >= low && code <= high {
			return true
		}
	}
	return false
}

func validateTargetContext(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("invalid target URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("target URL scheme must be http or https")
	}
	if u.User != nil {
		return errors.New("target URL credentials are not allowed")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return errors.New("invalid target host")
	}
	if host == "metadata.google.internal" || strings.HasSuffix(host, ".metadata.google.internal") {
		return errors.New("blocked metadata endpoint")
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}
	for _, ip := range ips {
		if err := validateIP(ip.Unmap()); err != nil {
			return err
		}
	}
	return nil
}

func validateIP(addr netip.Addr) error {
	if !addr.IsValid() || addr.IsUnspecified() || addr.IsLoopback() || addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() {
		return errors.New("target resolves to a prohibited address")
	}
	if addr.Is4() {
		b := addr.As4()
		if b[0] == 0 || b[0] == 127 || (b[0] == 100 && b[1] >= 64 && b[1] <= 127) ||
			(b[0] == 192 && b[1] == 0 && b[2] == 0) || (b[0] == 198 && (b[1] == 18 || b[1] == 19)) ||
			b[0] >= 224 {
			return errors.New("target resolves to a prohibited address")
		}
	}
	return nil
}

func sanitizeRequestError(err error) string {
	message := err.Error()
	if len(message) > 500 {
		message = message[:500]
	}
	return message
}
