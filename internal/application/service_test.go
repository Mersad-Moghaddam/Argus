package application

import (
	"context"
	"testing"
	"time"

	"argus/internal/models"
)

type mockIncidentStore struct {
	open              *models.Incident
	created           bool
	resolved          bool
	createdIncidentID int64
}

func (m *mockIncidentStore) GetOpenIncident(context.Context, int64) (*models.Incident, error) {
	return m.open, nil
}
func (m *mockIncidentStore) CreateIncident(context.Context, int64, string, time.Time) (int64, error) {
	m.created = true
	if m.createdIncidentID == 0 {
		m.createdIncidentID = 100
	}
	return m.createdIncidentID, nil
}
func (m *mockIncidentStore) ResolveIncident(context.Context, int64, time.Time) error {
	m.resolved = true
	return nil
}
func (m *mockIncidentStore) ListIncidents(context.Context, *int64, string, int, int) ([]models.Incident, error) {
	return nil, nil
}

type mockMaintenanceStore struct{ muted bool }

func (m *mockMaintenanceStore) CreateMaintenanceWindow(context.Context, models.MaintenanceWindow) (int64, error) {
	return 0, nil
}
func (m *mockMaintenanceStore) IsWebsiteMuted(context.Context, int64, time.Time) (bool, error) {
	return m.muted, nil
}

type mockOutboxStore struct{ added bool }

func (m *mockOutboxStore) AddEvent(context.Context, string, int64, string, []byte, time.Time) error {
	m.added = true
	return nil
}
func (m *mockOutboxStore) FetchPending(context.Context, int) ([]models.OutboxEvent, error) {
	return nil, nil
}
func (m *mockOutboxStore) MarkProcessed(context.Context, int64) error      { return nil }
func (m *mockOutboxStore) MarkFailed(context.Context, int64, string) error { return nil }

func TestProcessIncidentTransition_MutedStillTransitions(t *testing.T) {
	inc := &mockIncidentStore{}
	maint := &mockMaintenanceStore{muted: true}
	outbox := &mockOutboxStore{}
	s := &Service{incidents: inc, maintenance: maint, outbox: outbox}

	if err := s.ProcessIncidentTransition(context.Background(), 1, "https://example.com", "down", "failed", time.Now().UTC()); err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if !inc.created {
		t.Fatal("expected incident to be created even when muted")
	}
	if outbox.added {
		t.Fatal("expected outbox alert to be suppressed when muted")
	}
}

func TestProcessIncidentTransition_MutedResolveStillUpdatesIncident(t *testing.T) {
	openID := int64(44)
	inc := &mockIncidentStore{open: &models.Incident{ID: openID}}
	maint := &mockMaintenanceStore{muted: true}
	outbox := &mockOutboxStore{}
	s := &Service{incidents: inc, maintenance: maint, outbox: outbox}

	if err := s.ProcessIncidentTransition(context.Background(), 1, "https://example.com", "up", "", time.Now().UTC()); err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if !inc.resolved {
		t.Fatal("expected incident to be resolved even when muted")
	}
	if outbox.added {
		t.Fatal("expected outbox alert to be suppressed when muted")
	}
}
