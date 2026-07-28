package domain

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

// ValidationError gives API callers a durable machine code while preserving a
// plain English message for people. It unwraps ErrInvalidRoute so existing
// use-cases retain their error contract during migration.
type ValidationError struct {
	Code    string `json:"code"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string { return e.Message }
func (e *ValidationError) Unwrap() error { return ErrInvalidRoute }

func invalid(field, code, message string) error {
	return &ValidationError{Code: code, Field: field, Message: message}
}

// EndpointNormalization separates human input from the canonical identity
// used for persistence and from a concrete fetch URL. Query values never take
// part in the endpoint identity; a synthetic fixture must supply them later.
type EndpointNormalization struct {
	Method            string   `json:"method"`
	InputBaseURL      string   `json:"inputBaseUrl"`
	BaseURL           string   `json:"baseUrl"`
	RouteTemplate     string   `json:"routeTemplate"`
	CanonicalIdentity string   `json:"canonicalIdentity"`
	FetchTarget       string   `json:"fetchTarget,omitempty"`
	Changes           []string `json:"changes,omitempty"`
}

// NormalizeEndpoint is the only canonical path for a route's method, base
// URL, and route template. It deliberately performs no network access: SSRF
// policy remains enforced by the dial-time and redirect-time transport.
func NormalizeEndpoint(method, baseURL, routeTemplate string) (EndpointNormalization, error) {
	normalizedMethod, err := NormalizeMethod(method)
	if err != nil {
		return EndpointNormalization{}, err
	}
	path, err := NormalizeRouteTemplate(routeTemplate)
	if err != nil {
		return EndpointNormalization{}, err
	}
	base, baseChanges, err := NormalizeBaseURL(baseURL)
	if err != nil {
		return EndpointNormalization{}, err
	}
	out := EndpointNormalization{
		Method: normalizedMethod, InputBaseURL: baseURL, BaseURL: base,
		RouteTemplate: path, Changes: baseChanges,
	}
	if strings.TrimSpace(method) != normalizedMethod {
		out.Changes = append(out.Changes, "method_case_normalized")
	}
	if strings.TrimSpace(routeTemplate) != path {
		out.Changes = append(out.Changes, "route_template_normalized")
	}
	if base == "" {
		out.CanonicalIdentity = normalizedMethod + " " + path
		return out, nil
	}
	resolved, resolveErr := resolvePath(base, path)
	if resolveErr != nil {
		return EndpointNormalization{}, invalid("routeTemplate", "url_resolution_failed", "the route template could not be resolved against the base URL")
	}
	out.CanonicalIdentity = normalizedMethod + " " + resolved
	if !strings.Contains(path, "{") {
		out.FetchTarget = resolved
	}
	return out, nil
}

// NormalizeBaseURL accepts a structured, absolute HTTP(S) origin and optional
// base path. It rejects ambiguity rather than inventing a scheme or silently
// changing authority.
func NormalizeBaseURL(input string) (string, []string, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return "", nil, nil
	}
	if err := rejectUnsafeURLText(raw, "baseUrl"); err != nil {
		return "", nil, err
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() {
		return "", nil, invalid("baseUrl", "absolute_url_required", "the base URL must be an absolute HTTP or HTTPS URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", nil, invalid("baseUrl", "unsupported_scheme", "only HTTP and HTTPS base URLs are supported")
	}
	if u.User != nil {
		return "", nil, invalid("baseUrl", "userinfo_not_allowed", "base URLs must not include user information")
	}
	if u.Fragment != "" {
		return "", nil, invalid("baseUrl", "fragment_not_allowed", "base URLs must not include a fragment")
	}
	if u.RawQuery != "" {
		return "", nil, invalid("baseUrl", "query_not_allowed", "base URLs must not include query parameters")
	}
	host, port, err := canonicalHostAndPort(u)
	if err != nil {
		return "", nil, err
	}
	path, err := normalizeEscapedPath(u.EscapedPath(), "baseUrl")
	if err != nil {
		return "", nil, err
	}
	if path == "/" {
		path = ""
	} else {
		path = strings.TrimRight(path, "/")
	}
	canonicalHost := host
	if port != "" {
		canonicalHost = net.JoinHostPort(host, port)
	}
	canonical := u.Scheme + "://" + canonicalHost + path
	changes := make([]string, 0, 4)
	if raw != canonical {
		changes = append(changes, "base_url_normalized")
	}
	return canonical, changes, nil
}

// NormalizeRouteTemplate applies Argus's documented identity policy: repeated
// slashes are retained, while non-root trailing slashes are unified. Templates
// have no query or fragment; concrete query values belong to a synthetic
// fixture.
func NormalizeRouteTemplate(input string) (string, error) {
	raw := strings.TrimSpace(input)
	if err := rejectUnsafeURLText(raw, "routeTemplate"); err != nil {
		return "", err
	}
	if raw == "" {
		return "", invalid("routeTemplate", "route_required", "a route template is required")
	}
	if strings.ContainsAny(raw, "?#") {
		return "", invalid("routeTemplate", "query_or_fragment_not_allowed", "route templates must not include a query or fragment")
	}
	if !strings.HasPrefix(raw, "/") {
		raw = "/" + raw
	}
	raw = normalizeLegacyColonTemplate(raw)
	path, err := normalizeEscapedPath(raw, "routeTemplate")
	if err != nil {
		return "", err
	}
	if err := validateTemplateParameters(path); err != nil {
		return "", err
	}
	return path, nil
}

func normalizeLegacyColonTemplate(path string) string {
	var b strings.Builder
	b.Grow(len(path))
	for i := 0; i < len(path); {
		if path[i] != ':' || (i > 0 && path[i-1] != '/') {
			b.WriteByte(path[i])
			i++
			continue
		}
		end := strings.IndexByte(path[i:], '/')
		if end < 0 {
			end = len(path) - i
		}
		name := path[i+1 : i+end]
		if isTemplateName(name) {
			b.WriteByte('{')
			b.WriteString(name)
			b.WriteByte('}')
		} else {
			b.WriteString(path[i : i+end])
		}
		i += end
	}
	return b.String()
}

func rejectUnsafeURLText(value, field string) error {
	if !utf8.ValidString(value) {
		return invalid(field, "invalid_utf8", "the value is not valid UTF-8")
	}
	for _, r := range value {
		if r == '\\' || r == 0 || r == '\r' || r == '\n' || unicode.IsControl(r) || unicode.In(r, unicode.Bidi_Control) {
			return invalid(field, "unsafe_character", "the value contains a forbidden control or separator character")
		}
	}
	return nil
}

func canonicalHostAndPort(u *url.URL) (string, string, error) {
	host := u.Hostname()
	if host == "" {
		return "", "", invalid("baseUrl", "host_required", "the base URL must include a host")
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", "", invalid("baseUrl", "invalid_port", "the base URL port must be between 1 and 65535")
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	} else {
		host = strings.TrimSuffix(strings.ToLower(host), ".")
		ascii, err := idna.Lookup.ToASCII(host)
		if err != nil || ascii == "" {
			return "", "", invalid("baseUrl", "invalid_hostname", "the base URL host is not a valid domain name")
		}
		host = strings.ToLower(ascii)
	}
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	return host, port, nil
}

func normalizeEscapedPath(raw, field string) (string, error) {
	if raw == "" {
		return "/", nil
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '%' {
			continue
		}
		if i+2 >= len(raw) || !isHex(raw[i+1]) || !isHex(raw[i+2]) {
			return "", invalid(field, "malformed_percent_escape", "the path contains a malformed percent escape")
		}
		decoded := strings.ToLower(raw[i+1 : i+3])
		if decoded == "2f" || decoded == "5c" {
			return "", invalid(field, "encoded_separator_not_allowed", "encoded path separators are not allowed")
		}
		i += 2
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", invalid(field, "malformed_percent_escape", "the path contains a malformed percent escape")
	}
	if strings.Contains(decoded, "\\") {
		return "", invalid(field, "backslash_not_allowed", "paths must not include backslashes")
	}
	path, err := normalizePercentEncoding(raw)
	if err != nil {
		return "", err
	}
	path = removeDotSegments(path)
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	return path, nil
}

func normalizePercentEncoding(raw string) (string, error) {
	var b strings.Builder
	b.Grow(len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] != '%' {
			b.WriteByte(raw[i])
			continue
		}
		if i+2 >= len(raw) || !isHex(raw[i+1]) || !isHex(raw[i+2]) {
			return "", invalid("routeTemplate", "malformed_percent_escape", "the path contains a malformed percent escape")
		}
		value, _ := strconv.ParseUint(raw[i+1:i+3], 16, 8)
		ch := byte(value)
		if isUnreserved(ch) {
			b.WriteByte(ch)
		} else {
			b.WriteByte('%')
			b.WriteByte(strings.ToUpper(raw[i+1 : i+2])[0])
			b.WriteByte(strings.ToUpper(raw[i+2 : i+3])[0])
		}
		i += 2
	}
	return b.String(), nil
}

func isUnreserved(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9') || b == '-' || b == '.' || b == '_' || b == '~'
}

func isHex(b byte) bool {
	return ('0' <= b && b <= '9') || ('a' <= b && b <= 'f') || ('A' <= b && b <= 'F')
}

func removeDotSegments(path string) string {
	trailing := strings.HasSuffix(path, "/")
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		switch part {
		case ".":
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		default:
			out = append(out, part)
		}
	}
	result := strings.Join(out, "/")
	if !strings.HasPrefix(result, "/") {
		result = "/" + result
	}
	if trailing && result != "/" && !strings.HasSuffix(result, "/") {
		result += "/"
	}
	return result
}

func validateTemplateParameters(path string) error {
	for i := 0; i < len(path); {
		open := strings.IndexByte(path[i:], '{')
		close := strings.IndexByte(path[i:], '}')
		if open == -1 && close == -1 {
			return nil
		}
		if open == -1 || (close != -1 && close < open) {
			return invalid("routeTemplate", "invalid_template", "route template parameters must use balanced {name} placeholders")
		}
		start := i + open
		endOffset := strings.IndexByte(path[start+1:], '}')
		if endOffset == -1 {
			return invalid("routeTemplate", "invalid_template", "route template parameters must use balanced {name} placeholders")
		}
		end := start + 1 + endOffset
		name := path[start+1 : end]
		if name == "" || !isTemplateName(name) {
			return invalid("routeTemplate", "invalid_template_parameter", "route template parameter names must start with a letter and use letters, digits, underscores, or hyphens")
		}
		i = end + 1
	}
	return nil
}

func isTemplateName(value string) bool {
	for i, r := range value {
		if i == 0 && !unicode.IsLetter(r) {
			return false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

func resolvePath(base, route string) (string, error) {
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	// Route templates are stored with a leading slash for unambiguous display,
	// but are resolved as references relative to the configured base path.
	if u.Path != "" && !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}
	ref, err := url.Parse(strings.TrimPrefix(route, "/"))
	if err != nil {
		return "", err
	}
	// net/url correctly encodes template braces for wire URLs. Canonical
	// identities are template metadata, not wire targets, so keep braces
	// readable and distinct from a concrete escaped path.
	resolved := u.ResolveReference(ref).String()
	resolved = strings.ReplaceAll(resolved, "%7B", "{")
	resolved = strings.ReplaceAll(resolved, "%7D", "}")
	return resolved, nil
}

// ValidationCode returns a stable validation code when available.
func ValidationCode(err error) string {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return validation.Code
	}
	return "invalid_route"
}

// ValidationMessage gives callers a safe fallback that does not expose parse
// internals or secret-bearing input.
func ValidationMessage(err error) string {
	var validation *ValidationError
	if errors.As(err, &validation) {
		return validation.Message
	}
	return fmt.Sprintf("%s", ErrInvalidRoute)
}
