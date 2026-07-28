package api

import (
	"strconv"

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
