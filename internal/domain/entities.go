package domain

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

const (
	MonitorTypeHTTPStatus = "http_status"
	MonitorTypeKeyword    = "keyword"
	MonitorTypeHeartbeat  = "heartbeat"
	MonitorTypeTLSExpiry  = "tls_expiry"
)

var (
	ErrInvalidURL         = errors.New("invalid URL")
	ErrInvalidInterval    = errors.New("checkInterval must be at least 10 seconds")
	ErrInvalidMonitorType = errors.New("invalid monitorType")
	ErrInvalidInput       = errors.New("invalid input")
)

type Monitor struct {
	ID                     int64
	URL                    string
	HealthCheckURL         *string
	CheckIntervalSeconds   int
	MonitorType            string
	ExpectedKeyword        *string
	TLSExpiryThresholdDays int
	HeartbeatGraceSeconds  int
	Status                 string
	LastHeartbeatAt        *time.Time
	NextCheckAt            time.Time
	StatusPageID           *int64
}

type CheckResult struct {
	Status        string
	StatusCode    int
	LatencyMS     int
	FailureReason string
	CheckedAt     time.Time
}

type IncidentTransition struct {
	ShouldOpen    bool
	ShouldResolve bool
}

func NormalizeMonitor(input Monitor) (Monitor, error) {
	if input.CheckIntervalSeconds < 10 {
		return Monitor{}, ErrInvalidInterval
	}
	if input.MonitorType == "" {
		input.MonitorType = MonitorTypeHTTPStatus
	}
	switch input.MonitorType {
	case MonitorTypeHTTPStatus, MonitorTypeKeyword, MonitorTypeHeartbeat, MonitorTypeTLSExpiry:
	default:
		return Monitor{}, ErrInvalidMonitorType
	}

	normalizedURL, err := parseHTTPURL(input.URL)
	if err != nil {
		return Monitor{}, err
	}
	input.URL = normalizedURL

	if input.HealthCheckURL != nil && *input.HealthCheckURL != "" {
		normalizedHealthURL, parseErr := parseHTTPURL(*input.HealthCheckURL)
		if parseErr != nil {
			return Monitor{}, parseErr
		}
		input.HealthCheckURL = &normalizedHealthURL
	}

	if input.MonitorType == MonitorTypeKeyword && (input.ExpectedKeyword == nil || strings.TrimSpace(*input.ExpectedKeyword) == "") {
		return Monitor{}, errors.New("expectedKeyword is required for keyword monitor")
	}
	if input.MonitorType == MonitorTypeTLSExpiry && input.TLSExpiryThresholdDays <= 0 {
		input.TLSExpiryThresholdDays = 14
	}
	if input.MonitorType == MonitorTypeHeartbeat && input.HeartbeatGraceSeconds < 10 {
		input.HeartbeatGraceSeconds = input.CheckIntervalSeconds * 2
	}
	if input.Status == "" {
		input.Status = "pending"
	}
	if input.NextCheckAt.IsZero() {
		input.NextCheckAt = time.Now().UTC()
	}

	return input, nil
}

func IncidentPolicy(currentOpen bool, status string) IncidentTransition {
	if status == "down" && !currentOpen {
		return IncidentTransition{ShouldOpen: true}
	}
	if status == "up" && currentOpen {
		return IncidentTransition{ShouldResolve: true}
	}
	return IncidentTransition{}
}

func ShouldSuppressAlerts(inMaintenance bool) bool {
	return inMaintenance
}

func parseHTTPURL(raw string) (string, error) {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", ErrInvalidURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", ErrInvalidURL
	}
	return parsed.String(), nil
}
