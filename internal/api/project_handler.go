package api

import (
	"errors"
	"strconv"
	"time"

	"argus/internal/application"
	"argus/internal/domain"
	"argus/internal/models"

	"github.com/gofiber/fiber/v2"
)

type ProjectHandler struct{ service *application.Service }

func NewProjectHandler(service *application.Service) *ProjectHandler {
	return &ProjectHandler{service: service}
}

func RegisterProjectRoutes(app fiber.Router, h *ProjectHandler, guards ...fiber.Handler) {
	app.Get("/projects", guarded(guards, h.ListProjects)...)
	app.Post("/projects", guarded(guards, h.CreateProject)...)
	app.Get("/projects/:projectId", guarded(guards, h.GetProject)...)
	app.Put("/projects/:projectId", guarded(guards, h.UpdateProject)...)
	app.Post("/projects/:projectId/archive", guarded(guards, h.ArchiveProject)...)
	app.Post("/projects/:projectId/unarchive", guarded(guards, h.UnarchiveProject)...)
	app.Delete("/projects/:projectId", guarded(guards, h.DeleteProject)...)
	app.Get("/projects/:projectId/environments", guarded(guards, h.ListEnvironments)...)
	app.Post("/projects/:projectId/environments", guarded(guards, h.CreateEnvironment)...)
	app.Get("/projects/:projectId/telemetry-credentials", guarded(guards, h.ListTelemetryCredentials)...)
	app.Get("/projects/:projectId/telemetry-ingress", guarded(guards, h.ListTelemetryIngress)...)
	app.Post("/projects/:projectId/telemetry-credentials", guarded(guards, h.CreateTelemetryCredential)...)
	app.Post("/projects/:projectId/telemetry-credentials/:credentialId/rotate", guarded(guards, h.RotateTelemetryCredential)...)
	app.Post("/projects/:projectId/telemetry-credentials/:credentialId/revoke", guarded(guards, h.RevokeTelemetryCredential)...)
}

type environmentRequest struct {
	Name    string `json:"name"`
	BaseURL string `json:"baseUrl"`
}

func (h *ProjectHandler) ListEnvironments(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	items, err := h.service.ListProjectEnvironments(c.UserContext(), project.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list environments"})
	}
	return c.JSON(fiber.Map{"items": items})
}
func (h *ProjectHandler) CreateEnvironment(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	var req environmentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	env, err := h.service.CreateProjectEnvironment(c.UserContext(), project.ID, application.CreateEnvironmentInput{Name: req.Name, BaseURL: req.BaseURL})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(env)
}

type telemetryCredentialRequest struct {
	Name               string `json:"name"`
	EnvironmentID      int64  `json:"environmentId"`
	ExpiresInDays      int    `json:"expiresInDays"`
	RateLimitPerMinute int    `json:"rateLimitPerMinute"`
}

func telemetryCredentialInput(req telemetryCredentialRequest) (application.CreateTelemetryCredentialInput, error) {
	if req.ExpiresInDays < 0 || req.ExpiresInDays > 365 {
		return application.CreateTelemetryCredentialInput{}, domain.ErrInvalidInput
	}
	return application.CreateTelemetryCredentialInput{
		Name: req.Name, EnvironmentID: req.EnvironmentID, ExpiresIn: time.Duration(req.ExpiresInDays) * 24 * time.Hour,
		RateLimitPerMinute: req.RateLimitPerMinute,
	}, nil
}

func (h *ProjectHandler) ListTelemetryCredentials(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	items, err := h.service.ListTelemetryCredentials(c.UserContext(), project.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list telemetry credentials"})
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *ProjectHandler) ListTelemetryIngress(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	items, err := h.service.ListTelemetryIngress(c.UserContext(), project.ID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list telemetry diagnostics"})
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *ProjectHandler) CreateTelemetryCredential(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	var req telemetryCredentialRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	input, err := telemetryCredentialInput(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	issued, err := h.service.CreateTelemetryCredential(c.UserContext(), project.ID, currentUserID(c), input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(issued)
}

func (h *ProjectHandler) RotateTelemetryCredential(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	credentialID, err := parseIDParam(c, "credentialId")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var req telemetryCredentialRequest
	if err = c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	input, err := telemetryCredentialInput(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	issued, err := h.service.RotateTelemetryCredential(c.UserContext(), project.ID, currentUserID(c), credentialID, input)
	if errors.Is(err, application.ErrTelemetryCredentialNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "telemetry credential not found"})
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(issued)
}

func (h *ProjectHandler) RevokeTelemetryCredential(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	credentialID, err := parseIDParam(c, "credentialId")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	err = h.service.RevokeTelemetryCredential(c.UserContext(), project.ID, credentialID)
	if errors.Is(err, application.ErrTelemetryCredentialNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "telemetry credential not found"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke telemetry credential"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type projectRequest struct {
	Name                     string `json:"name"`
	Description              string `json:"description"`
	DefaultIntervalSeconds   int    `json:"defaultIntervalSeconds"`
	DefaultTimeoutMS         int    `json:"defaultTimeoutMs"`
	DefaultRetries           int    `json:"defaultRetries"`
	FailureThreshold         int    `json:"failureThreshold"`
	RecoverySuccessThreshold int    `json:"recoverySuccessThreshold"`
}

func (h *ProjectHandler) ListProjects(c *fiber.Ctx) error {
	userID := currentUserID(c)
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	items, total, err := h.service.ListProjects(c.UserContext(), userID, models.ProjectFilter{
		Search: c.Query("search"), Status: c.Query("status"), Limit: limit, Offset: offset,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list projects"})
	}
	return c.JSON(fiber.Map{"items": items, "total": total})
}

func (h *ProjectHandler) CreateProject(c *fiber.Ctx) error {
	var req projectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	userID := currentUserID(c)
	project, err := h.service.CreateProject(c.UserContext(), userID, application.CreateProjectInput{
		Name: req.Name, Description: req.Description, DefaultIntervalSeconds: req.DefaultIntervalSeconds,
		DefaultTimeoutMS: req.DefaultTimeoutMS, DefaultRetries: req.DefaultRetries,
		FailureThreshold: req.FailureThreshold, RecoverySuccessThreshold: req.RecoverySuccessThreshold,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(project)
}

func (h *ProjectHandler) GetProject(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	return c.JSON(project)
}

func (h *ProjectHandler) UpdateProject(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	var req projectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	updated, err := h.service.UpdateProject(c.UserContext(), project, application.UpdateProjectInput{
		Name: req.Name, Description: req.Description, DefaultIntervalSeconds: req.DefaultIntervalSeconds,
		DefaultTimeoutMS: req.DefaultTimeoutMS, DefaultRetries: req.DefaultRetries,
		FailureThreshold: req.FailureThreshold, RecoverySuccessThreshold: req.RecoverySuccessThreshold,
	})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(updated)
}

func (h *ProjectHandler) ArchiveProject(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleOwner)
	if !ok {
		return nil
	}
	if err := h.service.SetProjectStatus(c.UserContext(), project.ID, domain.ProjectStatusArchived); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to archive project"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ProjectHandler) UnarchiveProject(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleOwner)
	if !ok {
		return nil
	}
	if err := h.service.SetProjectStatus(c.UserContext(), project.ID, domain.ProjectStatusActive); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to unarchive project"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ProjectHandler) DeleteProject(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleOwner)
	if !ok {
		return nil
	}
	if err := h.service.DeleteProject(c.UserContext(), project.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete project"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
