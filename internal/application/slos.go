package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
)

var ErrSLODefinitionNotFound = errors.New("SLO definition not found")

const (
	defaultSLOWindowSeconds      = 30 * 24 * 60 * 60
	defaultSLOShortWindowSeconds = 60 * 60
	defaultSLOLongWindowSeconds  = 6 * 60 * 60
	defaultSLOShortBurnRate      = 14.4
	defaultSLOLongBurnRate       = 6.0
	maxSLOWindowSeconds          = 90 * 24 * 60 * 60
)

type CreateSLODefinitionInput struct {
	Name               string
	SLIKind            string
	TargetPercent      float64
	WindowSeconds      int
	LatencyThresholdMS int
	MinEvents          int
	ShortWindowSeconds int
	ShortBurnRate      float64
	LongWindowSeconds  int
	LongBurnRate       float64
}

// CreateSLODefinition creates version one and the matching immutable snapshot
// atomically in the store. Callers cannot create an SLO in another tenant.
func (s *Service) CreateSLODefinition(ctx context.Context, projectID, userID int64, input CreateSLODefinitionInput) (models.SLODefinition, error) {
	name := strings.TrimSpace(input.Name)
	if projectID <= 0 || userID <= 0 || name == "" || len(name) > 160 || input.TargetPercent <= 0 || input.TargetPercent >= 100 || input.MinEvents < 0 {
		return models.SLODefinition{}, domain.ErrInvalidInput
	}
	if input.WindowSeconds == 0 {
		input.WindowSeconds = defaultSLOWindowSeconds
	}
	if input.ShortWindowSeconds == 0 {
		input.ShortWindowSeconds = defaultSLOShortWindowSeconds
	}
	if input.LongWindowSeconds == 0 {
		input.LongWindowSeconds = defaultSLOLongWindowSeconds
	}
	if input.ShortBurnRate == 0 {
		input.ShortBurnRate = defaultSLOShortBurnRate
	}
	if input.LongBurnRate == 0 {
		input.LongBurnRate = defaultSLOLongBurnRate
	}
	if input.WindowSeconds < 60 || input.WindowSeconds > maxSLOWindowSeconds || input.ShortWindowSeconds < 60 || input.LongWindowSeconds < 60 || input.ShortWindowSeconds > input.LongWindowSeconds || input.LongWindowSeconds > input.WindowSeconds || input.ShortBurnRate <= 0 || input.LongBurnRate <= 0 {
		return models.SLODefinition{}, domain.ErrInvalidInput
	}
	if input.SLIKind != string(domain.SLIAvailability) && input.SLIKind != string(domain.SLILatency) {
		return models.SLODefinition{}, domain.ErrInvalidInput
	}
	if input.SLIKind == string(domain.SLILatency) && input.LatencyThresholdMS <= 0 {
		return models.SLODefinition{}, domain.ErrInvalidInput
	}
	if input.SLIKind == string(domain.SLIAvailability) && input.LatencyThresholdMS != 0 {
		return models.SLODefinition{}, domain.ErrInvalidInput
	}
	definition := models.SLODefinition{ProjectID: projectID, CreatedByUserID: userID, Name: name, SLIKind: input.SLIKind, TargetPercent: input.TargetPercent, WindowSeconds: input.WindowSeconds, LatencyThresholdMS: input.LatencyThresholdMS, MinEvents: input.MinEvents, ShortWindowSeconds: input.ShortWindowSeconds, ShortBurnRate: input.ShortBurnRate, LongWindowSeconds: input.LongWindowSeconds, LongBurnRate: input.LongBurnRate, Version: 1}
	id, err := s.slos.CreateSLODefinition(ctx, definition)
	if err != nil {
		return models.SLODefinition{}, err
	}
	persisted, err := s.slos.GetSLODefinition(ctx, projectID, id)
	if err != nil {
		return models.SLODefinition{}, err
	}
	if persisted == nil {
		return models.SLODefinition{}, ErrSLODefinitionNotFound
	}
	return *persisted, nil
}

func (s *Service) ListSLODefinitions(ctx context.Context, projectID int64) ([]models.SLODefinition, error) {
	return s.slos.ListSLODefinitions(ctx, projectID)
}

// RecordSLOEvaluation accepts only bounded aggregate evidence produced by a
// trusted evaluator. It verifies the project, definition, version, status and
// time window before persisting an explainable historical result.
func (s *Service) RecordSLOEvaluation(ctx context.Context, projectID, sloID int64, evaluation models.SLOEvaluation) (models.SLOEvaluation, error) {
	definition, err := s.slos.GetSLODefinition(ctx, projectID, sloID)
	if err != nil {
		return models.SLOEvaluation{}, err
	}
	if definition == nil || evaluation.GoodEvents < 0 || evaluation.TotalEvents < evaluation.GoodEvents || !validSLOStatus(evaluation.Status) || strings.TrimSpace(evaluation.Provenance) == "" || len(evaluation.Provenance) > 160 || evaluation.WindowStartedAt.IsZero() || evaluation.WindowEndedAt.Before(evaluation.WindowStartedAt) {
		return models.SLOEvaluation{}, domain.ErrInvalidInput
	}
	if evaluation.DefinitionVersion == 0 {
		evaluation.DefinitionVersion = definition.Version
	}
	if evaluation.DefinitionVersion != definition.Version {
		return models.SLOEvaluation{}, domain.ErrInvalidInput
	}
	if evaluation.EvaluatedAt.IsZero() {
		evaluation.EvaluatedAt = time.Now().UTC()
	}
	evaluation.ProjectID, evaluation.SLOID = projectID, sloID
	id, err := s.slos.RecordSLOEvaluation(ctx, evaluation)
	if err != nil {
		return models.SLOEvaluation{}, err
	}
	evaluation.ID = id
	return evaluation, nil
}

func (s *Service) ListSLOEvaluations(ctx context.Context, projectID, sloID int64, limit int) ([]models.SLOEvaluation, error) {
	definition, err := s.slos.GetSLODefinition(ctx, projectID, sloID)
	if err != nil {
		return nil, err
	}
	if definition == nil {
		return nil, ErrSLODefinitionNotFound
	}
	return s.slos.ListSLOEvaluations(ctx, projectID, sloID, limit)
}

func validSLOStatus(status string) bool {
	switch domain.SLOStatus(status) {
	case domain.SLOHealthy, domain.SLOUnhealthy, domain.SLONoData, domain.SLOStale, domain.SLOPaused, domain.SLOMaintenance, domain.SLOConfigurationError:
		return true
	default:
		return false
	}
}
