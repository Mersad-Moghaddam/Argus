package application

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
)

const (
	defaultHeartbeatInterval = 5 * time.Minute
	maxHeartbeatInterval     = 7 * 24 * time.Hour
	maxHeartbeatGrace        = 7 * 24 * time.Hour
)

var ErrHeartbeatMonitorNotFound = errors.New("heartbeat monitor not found")

type CreateHeartbeatMonitorInput struct {
	Name                    string
	EnvironmentID           int64
	ExpectedIntervalSeconds int
	GracePeriodSeconds      int
}

func (s *Service) CreateHeartbeatMonitor(ctx context.Context, projectID, userID int64, input CreateHeartbeatMonitorInput) (models.IssuedHeartbeatMonitor, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" || len(name) > 120 || input.EnvironmentID <= 0 || !s.projectHasEnvironment(ctx, projectID, input.EnvironmentID) {
		return models.IssuedHeartbeatMonitor{}, domain.ErrInvalidInput
	}
	interval := time.Duration(input.ExpectedIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = defaultHeartbeatInterval
	}
	if interval < time.Minute || interval > maxHeartbeatInterval {
		return models.IssuedHeartbeatMonitor{}, domain.ErrInvalidInput
	}
	grace := time.Duration(input.GracePeriodSeconds) * time.Second
	if grace <= 0 {
		grace = interval
	}
	if grace < time.Minute || grace > maxHeartbeatGrace {
		return models.IssuedHeartbeatMonitor{}, domain.ErrInvalidInput
	}
	raw, err := newHeartbeatToken()
	if err != nil {
		return models.IssuedHeartbeatMonitor{}, err
	}
	hash := sha256.Sum256([]byte(raw))
	monitor := models.HeartbeatMonitor{ProjectID: projectID, EnvironmentID: input.EnvironmentID, CreatedByUserID: userID, Name: name,
		TokenPrefix: raw[:16], TokenHash: hash[:], ExpectedIntervalSeconds: int(interval.Seconds()), GracePeriodSeconds: int(grace.Seconds())}
	id, err := s.heartbeats.CreateHeartbeatMonitor(ctx, monitor)
	if err != nil {
		return models.IssuedHeartbeatMonitor{}, err
	}
	monitor.ID = id
	s.logger.Add("info", "heartbeat", "heartbeat_monitor_created", "Heartbeat monitor created", nil, map[string]string{"projectId": strconv.FormatInt(projectID, 10), "monitorId": strconv.FormatInt(id, 10)})
	return models.IssuedHeartbeatMonitor{Monitor: monitor, Token: raw}, nil
}

func (s *Service) ListHeartbeatMonitors(ctx context.Context, projectID int64) ([]models.HeartbeatMonitor, error) {
	items, err := s.heartbeats.ListHeartbeatMonitors(ctx, projectID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].LastOutcome = heartbeatState(items[i], time.Now().UTC())
	}
	return items, nil
}

func (s *Service) RevokeHeartbeatMonitor(ctx context.Context, projectID, monitorID int64) error {
	monitor, err := s.heartbeats.GetHeartbeatMonitorByID(ctx, monitorID)
	if err != nil {
		return err
	}
	if monitor == nil || monitor.ProjectID != projectID {
		return ErrHeartbeatMonitorNotFound
	}
	return s.heartbeats.RevokeHeartbeatMonitor(ctx, monitorID, time.Now().UTC())
}

// ReceiveHeartbeat authenticates the project-bound opaque token and records a
// low-cardinality receipt. Replaying the same key is accepted but does not
// refresh liveness, which protects retries without extending a missed run.
func (s *Service) ReceiveHeartbeat(ctx context.Context, rawToken, idempotencyKey, outcome string) (*models.HeartbeatMonitor, bool, error) {
	rawToken, idempotencyKey, outcome = strings.TrimSpace(rawToken), strings.TrimSpace(idempotencyKey), strings.ToLower(strings.TrimSpace(outcome))
	if rawToken == "" || len(idempotencyKey) < 16 || len(idempotencyKey) > 200 || (outcome != "" && outcome != "success" && outcome != "failure") {
		return nil, false, ErrHeartbeatMonitorNotFound
	}
	if outcome == "" {
		outcome = "success"
	}
	hash := sha256.Sum256([]byte(rawToken))
	monitor, err := s.heartbeats.GetHeartbeatMonitorByHash(ctx, hash[:])
	if err != nil {
		return nil, false, err
	}
	if monitor == nil || monitor.RevokedAt != nil {
		return nil, false, ErrHeartbeatMonitorNotFound
	}
	now := time.Now().UTC()
	created, err := s.heartbeats.RecordHeartbeatReceipt(ctx, models.HeartbeatReceipt{MonitorID: monitor.ID, IdempotencyKey: idempotencyKey, Outcome: outcome, ReceivedAt: now})
	if err != nil {
		return nil, false, err
	}
	if created {
		if err = s.heartbeats.TouchHeartbeatMonitor(ctx, monitor.ID, now, outcome); err != nil {
			return nil, false, err
		}
		monitor.LastReceivedAt, monitor.LastOutcome = &now, outcome
	}
	return monitor, created, nil
}

func heartbeatState(monitor models.HeartbeatMonitor, now time.Time) string {
	if monitor.RevokedAt != nil {
		return "revoked"
	}
	if monitor.LastReceivedAt == nil {
		return "missing"
	}
	age := now.Sub(*monitor.LastReceivedAt)
	if age <= time.Duration(monitor.ExpectedIntervalSeconds)*time.Second {
		return "healthy"
	}
	if age <= time.Duration(monitor.ExpectedIntervalSeconds+monitor.GracePeriodSeconds)*time.Second {
		return "late"
	}
	return "missing"
}

func newHeartbeatToken() (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return "argus_hb_" + base64.RawURLEncoding.EncodeToString(secret), nil
}
