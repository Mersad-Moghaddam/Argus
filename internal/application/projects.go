package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"argus/internal/domain"
	"argus/internal/models"
)

type CreateEnvironmentInput struct{ Name, BaseURL string }

func (s *Service) ListProjectEnvironments(ctx context.Context, projectID int64) ([]models.ProjectEnvironment, error) {
	return s.projects.ListProjectEnvironments(ctx, projectID)
}

func (s *Service) CreateProjectEnvironment(ctx context.Context, projectID int64, input CreateEnvironmentInput) (models.ProjectEnvironment, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.ProjectEnvironment{}, domain.ErrInvalidInput
	}
	base, _, err := domain.NormalizeBaseURL(input.BaseURL)
	if err != nil && strings.TrimSpace(input.BaseURL) != "" {
		return models.ProjectEnvironment{}, err
	}
	env := models.ProjectEnvironment{ProjectID: projectID, Name: name, CanonicalBaseURL: base}
	id, err := s.projects.CreateProjectEnvironment(ctx, env)
	if err != nil {
		return models.ProjectEnvironment{}, err
	}
	env.ID = id
	return env, nil
}

var (
	ErrProjectNameRequired = errors.New("project name is required")
	ErrInsufficientRole    = errors.New("insufficient permissions for this action")
)

var slugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

// roleRank orders roles from least to most privileged so authorization
// checks can express "at least editor" etc.
var roleRank = map[string]int{models.ProjectRoleViewer: 1, models.ProjectRoleEditor: 2, models.ProjectRoleOwner: 3}

type CreateProjectInput struct {
	Name                     string
	Description              string
	DefaultIntervalSeconds   int
	DefaultTimeoutMS         int
	DefaultRetries           int
	FailureThreshold         int
	RecoverySuccessThreshold int
}

func (s *Service) CreateProject(ctx context.Context, ownerUserID int64, input CreateProjectInput) (models.Project, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.Project{}, ErrProjectNameRequired
	}
	project := models.Project{
		Name:                     name,
		Slug:                     slugify(name),
		Description:              strings.TrimSpace(input.Description),
		Status:                   domain.ProjectStatusActive,
		DefaultIntervalSeconds:   orDefault(input.DefaultIntervalSeconds, 300, 10, 86400),
		DefaultTimeoutMS:         orDefault(input.DefaultTimeoutMS, 5000, 200, 60000),
		DefaultRetries:           orDefault(input.DefaultRetries, 1, 0, 5),
		FailureThreshold:         orDefault(input.FailureThreshold, domain.DefaultFailureThreshold, 1, 20),
		RecoverySuccessThreshold: orDefault(input.RecoverySuccessThreshold, domain.DefaultRecoverySuccesses, 1, 20),
	}
	id, err := s.projects.CreateProject(ctx, project, ownerUserID)
	if err != nil {
		return models.Project{}, err
	}
	project.ID = id
	project.OwnerUserID = ownerUserID
	s.logger.Add("info", "api", "project_created", "Project created", nil, map[string]string{"projectId": strconv.FormatInt(id, 10), "name": name})
	return project, nil
}

type UpdateProjectInput struct {
	Name                     string
	Description              string
	DefaultIntervalSeconds   int
	DefaultTimeoutMS         int
	DefaultRetries           int
	FailureThreshold         int
	RecoverySuccessThreshold int
}

func (s *Service) UpdateProject(ctx context.Context, project models.Project, input UpdateProjectInput) (models.Project, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return models.Project{}, ErrProjectNameRequired
	}
	project.Name = name
	project.Description = strings.TrimSpace(input.Description)
	project.DefaultIntervalSeconds = orDefault(input.DefaultIntervalSeconds, project.DefaultIntervalSeconds, 10, 86400)
	project.DefaultTimeoutMS = orDefault(input.DefaultTimeoutMS, project.DefaultTimeoutMS, 200, 60000)
	project.DefaultRetries = orDefault(input.DefaultRetries, project.DefaultRetries, 0, 5)
	project.FailureThreshold = orDefault(input.FailureThreshold, project.FailureThreshold, 1, 20)
	project.RecoverySuccessThreshold = orDefault(input.RecoverySuccessThreshold, project.RecoverySuccessThreshold, 1, 20)
	if err := s.projects.UpdateProject(ctx, project); err != nil {
		return models.Project{}, err
	}
	return project, nil
}

func (s *Service) SetProjectStatus(ctx context.Context, projectID int64, status string) error {
	if status != domain.ProjectStatusActive && status != domain.ProjectStatusArchived {
		return domain.ErrInvalidInput
	}
	return s.projects.SetProjectStatus(ctx, projectID, status)
}

func (s *Service) DeleteProject(ctx context.Context, projectID int64) error {
	return s.projects.DeleteProject(ctx, projectID)
}

func (s *Service) GetProject(ctx context.Context, projectID int64) (*models.Project, error) {
	return s.projects.GetProjectByID(ctx, projectID)
}

func (s *Service) ListProjects(ctx context.Context, userID int64, filter models.ProjectFilter) ([]models.Project, int, error) {
	return s.projects.ListProjects(ctx, userID, filter)
}

// AuthorizeProject loads a project and enforces that the given user is a
// member with at least minRole. Both "project does not exist" and "user is
// not a member" are surfaced identically as domain.ErrProjectNotFound so
// handlers can return 404 for both, avoiding project-ID enumeration.
func (s *Service) AuthorizeProject(ctx context.Context, projectID, userID int64, minRole string) (*models.Project, string, error) {
	project, err := s.projects.GetProjectByID(ctx, projectID)
	if err != nil {
		return nil, "", err
	}
	if project == nil {
		return nil, "", domain.ErrProjectNotFound
	}
	member, err := s.projects.GetProjectMember(ctx, projectID, userID)
	if err != nil {
		return nil, "", err
	}
	if member == nil {
		return nil, "", domain.ErrProjectNotFound
	}
	if roleRank[member.Role] < roleRank[minRole] {
		return nil, "", ErrInsufficientRole
	}
	project.ViewerRole = member.Role
	return project, member.Role, nil
}

func slugify(name string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	base = slugSanitizer.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "project"
	}
	suffix := make([]byte, 3)
	_, _ = rand.Read(suffix)
	return base + "-" + hex.EncodeToString(suffix)
}

func orDefault(value, fallback, min, max int) int {
	if value <= 0 {
		value = fallback
	}
	if value < min {
		value = min
	}
	if value > max {
		value = max
	}
	return value
}
