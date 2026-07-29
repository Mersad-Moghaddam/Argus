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
	"strconv"
	"strings"
	"syscall"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
	"argus/internal/openapi"
)

// Evaluator limits. Monitored URLs are fully untrusted user input, so every
// outbound request is bounded in time, size and redirect depth.
const (
	// MaxRouteResponseBytes caps how much of a monitored response body is
	// read. Bodies are never stored; only drained so connections can be
	// reused, so a small cap is enough.
	MaxRouteResponseBytes = 1 << 20 // 1MB
	// MaxRouteRedirects caps redirect hops. Every hop is re-validated
	// against the same address policy because redirects are the classic
	// SSRF bypass.
	MaxRouteRedirects = 5
	// MaxRouteTimeout is the hard ceiling applied on top of the per-route
	// timeout so a misconfigured route cannot occupy a worker slot forever.
	MaxRouteTimeout = 60 * time.Second
	// MinRouteTimeout guards against a zero/absurdly small configured value.
	MinRouteTimeout = 200 * time.Millisecond
	// MaxRouteRetries caps configured retries regardless of stored value.
	MaxRouteRetries = 5
	// retryBackoffBase is the first retry delay; it doubles per attempt and
	// is capped by retryBackoffMax.
	retryBackoffBase = 200 * time.Millisecond
	retryBackoffMax  = 2 * time.Second
)

// ErrBlockedTarget is returned when a target address violates the network
// policy. It is deliberately distinguishable so callers can report a clear,
// non-leaky reason.
var ErrBlockedTarget = errors.New("blocked target address")

// metadataHostnames are cloud instance-metadata hostnames that are blocked
// regardless of the private-network policy, because reaching them is never a
// legitimate monitoring use case and is a well-known credential-theft vector.
var metadataHostnames = map[string]bool{
	"metadata.google.internal":     true,
	"metadata.goog":                true,
	"instance-data":                true,
	"metadata":                     true,
	"metadata.azure.com":           true,
	"169.254.169.254":              true,
	"fd00:ec2::254":                true,
	"metadata.packet.net":          true,
	"metadata.platformequinix.com": true,
}

// blockedPrefixes are ranges that must never be dialed even when the
// private-network policy is set to allow internal targets. They cover cloud
// metadata services, CGNAT, benchmarking and documentation ranges.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("169.254.0.0/16"), // link-local incl. 169.254.169.254
	netip.MustParsePrefix("fd00:ec2::/64"),  // AWS IMDS over IPv6
	netip.MustParsePrefix("fe80::/10"),      // IPv6 link-local
	netip.MustParsePrefix("0.0.0.0/8"),      // "this network"
	netip.MustParsePrefix("100.64.0.0/10"),  // CGNAT
	netip.MustParsePrefix("192.0.0.0/24"),   // IETF protocol assignments
	netip.MustParsePrefix("198.18.0.0/15"),  // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),    // reserved
	netip.MustParsePrefix("2001:db8::/32"),  // documentation
}

// EvaluatorConfig configures the outbound-request policy for route checks.
type EvaluatorConfig struct {
	// AllowPrivateTargets relaxes the private/loopback address block so an
	// operator can monitor APIs on their own internal network. It defaults
	// to false (block). Cloud metadata endpoints stay blocked either way.
	AllowPrivateTargets bool
	// MaxRedirects caps redirect hops (0 uses MaxRouteRedirects).
	MaxRedirects int
	// MaxResponseBytes caps the drained response body (0 uses the default).
	MaxResponseBytes int64
	// MaxTimeout is the ceiling for per-route timeouts (0 uses the default).
	MaxTimeout time.Duration
	// UserAgent identifies Argus to monitored services.
	UserAgent string
}

func (c EvaluatorConfig) withDefaults() EvaluatorConfig {
	if c.MaxRedirects <= 0 {
		c.MaxRedirects = MaxRouteRedirects
	}
	if c.MaxResponseBytes <= 0 {
		c.MaxResponseBytes = MaxRouteResponseBytes
	}
	if c.MaxTimeout <= 0 || c.MaxTimeout > MaxRouteTimeout {
		c.MaxTimeout = MaxRouteTimeout
	}
	if strings.TrimSpace(c.UserAgent) == "" {
		c.UserAgent = "Argus-Monitor/1.0"
	}
	return c
}

// RouteEvaluator performs a single monitored HTTP request for an API route.
// One instance is shared by all check workers: it owns a connection-pooling
// transport whose dialer re-validates the *resolved* IP of every connection
// (including redirect hops), which closes the DNS-rebinding hole that a
// hostname-only pre-flight check would leave open.
type RouteEvaluator struct {
	cfg    EvaluatorConfig
	client *http.Client
}

// RouteCheckOutcome is the result of evaluating one route.
type RouteCheckOutcome struct {
	Status        string // "up" or "down"
	StatusCode    int
	LatencyMS     int
	FailureReason string
	Attempts      int
}

// NewRouteEvaluator builds an evaluator with a hardened HTTP client.
func NewRouteEvaluator(cfg EvaluatorConfig) *RouteEvaluator {
	cfg = cfg.withDefaults()
	e := &RouteEvaluator{cfg: cfg}
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control:   e.controlConnection,
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          200,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: cfg.MaxTimeout,
		DisableCompression:    false,
	}
	e.client = &http.Client{
		Transport:     transport,
		CheckRedirect: e.checkRedirect,
	}
	return e
}

// controlConnection runs after DNS resolution and immediately before the
// socket connects, so it sees the exact IP that will be contacted. This makes
// the address policy immune to DNS rebinding and to redirect chains.
func (e *RouteEvaluator) controlConnection(network, address string, _ syscall.RawConn) error {
	if network != "tcp4" && network != "tcp6" && network != "tcp" {
		return fmt.Errorf("%w: unsupported network %q", ErrBlockedTarget, network)
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%w: unparsable address", ErrBlockedTarget)
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("%w: unresolvable address", ErrBlockedTarget)
	}
	return e.validateAddr(addr)
}

// validateAddr applies the network policy to a resolved IP address.
func (e *RouteEvaluator) validateAddr(addr netip.Addr) error {
	a := addr.Unmap()
	if !a.IsValid() {
		return fmt.Errorf("%w: invalid address", ErrBlockedTarget)
	}
	if a.IsUnspecified() {
		return fmt.Errorf("%w: unspecified address", ErrBlockedTarget)
	}
	if a.IsMulticast() || a.IsInterfaceLocalMulticast() || a.IsLinkLocalMulticast() {
		return fmt.Errorf("%w: %s is a multicast address", ErrBlockedTarget, a)
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return fmt.Errorf("%w: %s is in a reserved range", ErrBlockedTarget, a)
		}
	}
	if e.cfg.AllowPrivateTargets {
		return nil
	}
	if a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() {
		return fmt.Errorf("%w: %s is a private or loopback address", ErrBlockedTarget, a)
	}
	return nil
}

// validateRequestURL performs the cheap, pre-flight structural checks that do
// not need DNS: scheme allow-list, host presence, no embedded credentials and
// a literal-IP check so obviously-blocked targets fail before any I/O.
func (e *RouteEvaluator) validateRequestURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return nil, fmt.Errorf("%w: scheme %q is not allowed", ErrBlockedTarget, u.Scheme)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: embedded credentials are not allowed", ErrBlockedTarget)
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("invalid URL: missing host")
	}
	if metadataHostnames[strings.ToLower(strings.TrimSuffix(host, "."))] {
		return nil, fmt.Errorf("%w: cloud metadata endpoint", ErrBlockedTarget)
	}
	if port := u.Port(); port != "" {
		n, convErr := strconv.Atoi(port)
		if convErr != nil || n <= 0 || n > 65535 {
			return nil, fmt.Errorf("invalid URL: bad port %q", port)
		}
	}
	if addr, addrErr := netip.ParseAddr(host); addrErr == nil {
		if err = e.validateAddr(addr); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// checkRedirect caps hop count and re-validates every redirect target. The
// dialer validates the resolved IP too; this layer additionally enforces the
// scheme allow-list and metadata-hostname block on each hop and produces a
// clearer failure reason than a raw dial error.
func (e *RouteEvaluator) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= e.cfg.MaxRedirects {
		return fmt.Errorf("stopped after %d redirects", e.cfg.MaxRedirects)
	}
	if _, err := e.validateRequestURL(req.URL.String()); err != nil {
		return fmt.Errorf("redirect rejected: %w", err)
	}
	// Never forward configured secrets across a redirect to another origin.
	if len(via) > 0 && !sameOrigin(via[0].URL, req.URL) {
		req.Header.Del("Authorization")
		req.Header.Del("Cookie")
		req.Header.Del("X-Api-Key")
		req.Header.Del("X-Auth-Token")
		req.Header.Del("Proxy-Authorization")
	}
	return nil
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

// EvaluateRoute runs one monitored request for a route, honouring its
// timeout, retries and expected status range. It never returns an error: a
// failure to reach the target *is* the result.
func (e *RouteEvaluator) EvaluateRoute(ctx context.Context, route models.APIRoute) RouteCheckOutcome {
	target, err := BuildRouteURL(route)
	if err != nil {
		return RouteCheckOutcome{Status: "down", FailureReason: err.Error(), Attempts: 1}
	}
	if _, err = e.validateRequestURL(target); err != nil {
		// Fail closed without ever opening a socket.
		return RouteCheckOutcome{Status: "down", FailureReason: err.Error(), Attempts: 1}
	}

	expected, err := ParseStatusRange(route.ExpectedStatusRange)
	if err != nil {
		return RouteCheckOutcome{Status: "down", FailureReason: err.Error(), Attempts: 1}
	}
	headers := parseRouteHeaders(route.Headers)
	timeout := clampTimeout(route.TimeoutMS, e.cfg.MaxTimeout)
	retries := route.Retries
	if retries < 0 {
		retries = 0
	}
	if retries > MaxRouteRetries {
		retries = MaxRouteRetries
	}

	var outcome RouteCheckOutcome
	for attempt := 1; attempt <= retries+1; attempt++ {
		outcome = e.attempt(ctx, route.Method, target, headers, timeout, expected)
		outcome.Attempts = attempt
		if outcome.Status == "up" {
			return outcome
		}
		// A blocked target is a permanent policy decision; retrying it only
		// burns worker slots.
		if strings.Contains(outcome.FailureReason, ErrBlockedTarget.Error()) {
			return outcome
		}
		if attempt <= retries {
			if err = sleepCtx(ctx, backoffFor(attempt)); err != nil {
				outcome.FailureReason = "check cancelled"
				return outcome
			}
		}
	}
	return outcome
}

func (e *RouteEvaluator) attempt(ctx context.Context, method, target string, headers map[string]string, timeout time.Duration, expected statusRange) RouteCheckOutcome {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, target, nil)
	if err != nil {
		return RouteCheckOutcome{Status: "down", FailureReason: "invalid request: " + err.Error()}
	}
	req.Header.Set("User-Agent", e.cfg.UserAgent)
	req.Header.Set("Accept", "*/*")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := e.client.Do(req)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return RouteCheckOutcome{Status: "down", LatencyMS: latency, FailureReason: describeTransportError(err, timeout)}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, e.cfg.MaxResponseBytes))

	if expected.contains(resp.StatusCode) {
		return RouteCheckOutcome{Status: "up", StatusCode: resp.StatusCode, LatencyMS: latency}
	}
	return RouteCheckOutcome{
		Status: "down", StatusCode: resp.StatusCode, LatencyMS: latency,
		FailureReason: fmt.Sprintf("status %d outside expected range %s", resp.StatusCode, expected.String()),
	}
}

// describeTransportError turns a transport failure into a short, stable and
// non-leaky reason string. url.Error wrapping is unwrapped so a blocked-target
// dial rejection is still recognisable to the retry logic.
func describeTransportError(err error, timeout time.Duration) string {
	if errors.Is(err, ErrBlockedTarget) {
		var inner error = err
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			inner = urlErr.Err
		}
		msg := inner.Error()
		if idx := strings.Index(msg, ErrBlockedTarget.Error()); idx > 0 {
			msg = msg[idx:]
		}
		return msg
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeoutError(err) {
		return fmt.Sprintf("request timed out after %s", timeout)
	}
	if errors.Is(err, context.Canceled) {
		return "check cancelled"
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err.Error()
	}
	return err.Error()
}

func isTimeoutError(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func backoffFor(attempt int) time.Duration {
	d := retryBackoffBase << (attempt - 1)
	if d > retryBackoffMax {
		return retryBackoffMax
	}
	return d
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func clampTimeout(timeoutMS int, max time.Duration) time.Duration {
	d := time.Duration(timeoutMS) * time.Millisecond
	if d < MinRouteTimeout {
		d = MinRouteTimeout
	}
	if d > max {
		d = max
	}
	return d
}

// parseRouteHeaders decodes the route's stored headers JSON. Malformed JSON
// yields no headers rather than an error: the check should still run.
func parseRouteHeaders(raw string) map[string]string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil
	}
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		k = strings.TrimSpace(k)
		// Hop-by-hop and length headers are owned by the transport.
		switch strings.ToLower(k) {
		case "", "host", "content-length", "connection", "transfer-encoding":
			continue
		}
		out[k] = v
	}
	return out
}

// BuildRouteURL joins a route's base URL and path, substituting any path
// template parameters (`{id}`, `:id`) with the parameter's stored example or
// default, falling back to a type-appropriate synthetic value so templated
// routes remain dispatchable.
func BuildRouteURL(route models.APIRoute) (string, error) {
	if strings.TrimSpace(route.BaseURL) == "" {
		return "", errors.New("route has no base URL configured")
	}
	method := route.Method
	if method == "" {
		method = "GET"
	}
	normalized, err := domain.NormalizeEndpoint(method, route.BaseURL, route.Path)
	if err != nil {
		return "", err
	}
	resolvedPath := substitutePathParams(normalized.RouteTemplate, decodeParameters(route.Parameters))
	base, err := url.Parse(normalized.BaseURL)
	if err != nil {
		return "", err
	}
	if base.Path != "" && !strings.HasSuffix(base.Path, "/") {
		base.Path += "/"
	}
	ref, err := url.Parse(strings.TrimPrefix(resolvedPath, "/"))
	if err != nil {
		return "", err
	}
	return base.ResolveReference(ref).String(), nil
}

func decodeParameters(raw string) []openapi.Parameter {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var params []openapi.Parameter
	if err := json.Unmarshal([]byte(raw), &params); err != nil {
		return nil
	}
	return params
}

// substitutePathParams replaces `{name}` and `:name` placeholders. Values are
// URL-path-escaped so a crafted example value cannot inject extra path
// segments, a query string or a new authority into the request URL.
func substitutePathParams(path string, params []openapi.Parameter) string {
	byName := map[string]openapi.Parameter{}
	for _, p := range params {
		if strings.EqualFold(p.In, "path") {
			byName[p.Name] = p
		}
	}
	var b strings.Builder
	b.Grow(len(path))
	for i := 0; i < len(path); {
		switch {
		case path[i] == '{':
			end := strings.IndexByte(path[i:], '}')
			if end < 0 {
				b.WriteString(path[i:])
				return b.String()
			}
			name := path[i+1 : i+end]
			b.WriteString(url.PathEscape(pathParamValue(name, byName[name])))
			i += end + 1
		case path[i] == ':' && (i == 0 || path[i-1] == '/'):
			end := strings.IndexByte(path[i:], '/')
			if end < 0 {
				end = len(path) - i
			}
			name := path[i+1 : i+end]
			b.WriteString(url.PathEscape(pathParamValue(name, byName[name])))
			i += end
		default:
			b.WriteByte(path[i])
			i++
		}
	}
	return b.String()
}

// pathParamValue picks the most faithful stand-in for a path parameter:
// the spec's example, then its default, then a type-appropriate placeholder.
func pathParamValue(name string, param openapi.Parameter) string {
	if v := strings.TrimSpace(param.Example); v != "" {
		return v
	}
	if v := strings.TrimSpace(param.Default); v != "" {
		return v
	}
	switch strings.ToLower(param.Type) {
	case "integer", "number":
		return "1"
	case "boolean":
		return "true"
	case "string", "":
		if strings.Contains(strings.ToLower(name), "id") {
			return "1"
		}
		return "sample"
	default:
		return "1"
	}
}

// statusRange is a parsed expected-status expression.
type statusRange struct {
	raw    string
	ranges [][2]int
}

func (s statusRange) contains(code int) bool {
	for _, r := range s.ranges {
		if code >= r[0] && code <= r[1] {
			return true
		}
	}
	return false
}

func (s statusRange) String() string { return s.raw }

// ParseStatusRange parses expressions such as "200-399", "200,201,204" or
// "200-204,301". An empty expression defaults to 200-399.
func ParseStatusRange(expr string) (statusRange, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		expr = "200-399"
	}
	out := statusRange{raw: expr}
	for _, part := range strings.Split(expr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lo, hi, found := strings.Cut(part, "-")
		low, err := strconv.Atoi(strings.TrimSpace(lo))
		if err != nil {
			return statusRange{}, fmt.Errorf("invalid expected status range %q", expr)
		}
		high := low
		if found {
			high, err = strconv.Atoi(strings.TrimSpace(hi))
			if err != nil {
				return statusRange{}, fmt.Errorf("invalid expected status range %q", expr)
			}
		}
		if low < 100 || high > 599 || low > high {
			return statusRange{}, fmt.Errorf("invalid expected status range %q", expr)
		}
		out.ranges = append(out.ranges, [2]int{low, high})
	}
	if len(out.ranges) == 0 {
		return statusRange{}, fmt.Errorf("invalid expected status range %q", expr)
	}
	return out, nil
}
