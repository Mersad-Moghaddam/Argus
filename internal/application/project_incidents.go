package application

import (
	"context"
	"time"

	"argus/internal/models"
)

func (s *Service) ListProjectIncidents(ctx context.Context, projectID int64, state string, limit, offset int) ([]models.ProjectIncident, error) {
	if s.projectIncidents == nil {
		return []models.ProjectIncident{}, nil
	}
	return s.projectIncidents.ListProjectIncidents(ctx, projectID, state, limit, offset)
}

func (s *Service) AcknowledgeProjectIncident(ctx context.Context, projectID, incidentID, userID int64) error {
	if s.projectIncidents == nil {
		return nil
	}
	return s.projectIncidents.AcknowledgeProjectIncident(ctx, projectID, incidentID, userID, time.Now().UTC())
}
