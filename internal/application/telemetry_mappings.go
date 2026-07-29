package application

import (
	"context"
	"strings"

	"argus/internal/domain"
	"argus/internal/models"
)

type CreateTelemetryRouteMappingInput struct {
	EnvironmentID, RouteID             int64
	ServiceName, DeploymentEnvironment string
}

func (s *Service) CreateTelemetryRouteMapping(ctx context.Context, projectID int64, input CreateTelemetryRouteMappingInput) (models.TelemetryRouteMapping, error) {
	serviceName := strings.TrimSpace(input.ServiceName)
	if input.EnvironmentID <= 0 || input.RouteID <= 0 || serviceName == "" || len(serviceName) > 160 {
		return models.TelemetryRouteMapping{}, domain.ErrInvalidInput
	}
	if !s.projectHasEnvironment(ctx, projectID, input.EnvironmentID) {
		return models.TelemetryRouteMapping{}, domain.ErrInvalidInput
	}
	route, err := s.routes.GetRouteByID(ctx, input.RouteID)
	if err != nil {
		return models.TelemetryRouteMapping{}, err
	}
	if route == nil || route.ProjectID != projectID {
		return models.TelemetryRouteMapping{}, ErrTelemetryCredentialNotFound
	}
	mapping := models.TelemetryRouteMapping{ProjectID: projectID, EnvironmentID: input.EnvironmentID, RouteID: route.ID, ServiceName: serviceName, DeploymentEnvironment: strings.TrimSpace(input.DeploymentEnvironment), HTTPMethod: route.Method, RouteTemplate: route.Path, Source: "manual"}
	id, err := s.telemetryMappings.CreateTelemetryRouteMapping(ctx, mapping)
	if err != nil {
		return models.TelemetryRouteMapping{}, err
	}
	mapping.ID = id
	return mapping, nil
}
func (s *Service) ListTelemetryRouteMappings(ctx context.Context, projectID int64) ([]models.TelemetryRouteMapping, error) {
	return s.telemetryMappings.ListTelemetryRouteMappings(ctx, projectID)
}
func (s *Service) DeleteTelemetryRouteMapping(ctx context.Context, projectID, id int64) error {
	return s.telemetryMappings.DeleteTelemetryRouteMapping(ctx, projectID, id)
}
