package api

import (
	"errors"
	"strings"

	"argus/internal/application"
	"argus/internal/domain"
	"argus/internal/models"

	"github.com/gofiber/fiber/v2"
)

// HeartbeatHandler separates authenticated heartbeat administration from the
// token-authenticated public receive path. Browser sessions never authorize a
// job ping and a job token cannot access project management APIs.
type HeartbeatHandler struct{ service *application.Service }

func NewHeartbeatHandler(service *application.Service) *HeartbeatHandler {
	return &HeartbeatHandler{service: service}
}

func RegisterHeartbeatRoutes(app fiber.Router, h *HeartbeatHandler, guards ...fiber.Handler) {
	app.Get("/heartbeat/catalog/:projectId", guarded(guards, h.List)...)
	app.Post("/heartbeat/catalog/:projectId", guarded(guards, h.Create)...)
	app.Post("/heartbeat/revoke/:projectId/:monitorId", guarded(guards, h.Revoke)...)
	app.Post("/heartbeat/ping", h.Receive)
}

type heartbeatMonitorRequest struct {
	Name                    string `json:"name"`
	EnvironmentID           int64  `json:"environmentId"`
	ExpectedIntervalSeconds int    `json:"expectedIntervalSeconds"`
	GracePeriodSeconds      int    `json:"gracePeriodSeconds"`
}
type heartbeatPingRequest struct {
	Outcome string `json:"outcome"`
}

func (h *HeartbeatHandler) List(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	items, err := h.service.ListHeartbeatMonitors(c.UserContext(), project.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list heartbeats"})
	}
	return c.JSON(fiber.Map{"items": items})
}
func (h *HeartbeatHandler) Create(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	var req heartbeatMonitorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	issued, err := h.service.CreateHeartbeatMonitor(c.UserContext(), project.ID, currentUserID(c), application.CreateHeartbeatMonitorInput{Name: req.Name, EnvironmentID: req.EnvironmentID, ExpectedIntervalSeconds: req.ExpectedIntervalSeconds, GracePeriodSeconds: req.GracePeriodSeconds})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(issued)
}
func (h *HeartbeatHandler) Revoke(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	monitorID, err := parseIDParam(c, "monitorId")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err = h.service.RevokeHeartbeatMonitor(c.UserContext(), project.ID, monitorID); errors.Is(err, application.ErrHeartbeatMonitorNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "heartbeat monitor not found"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke heartbeat monitor"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *HeartbeatHandler) Receive(c *fiber.Ctx) error {
	if c.Request().Header.ContentLength() > 4*1024 {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "heartbeat payload is too large"})
	}
	var req heartbeatPingRequest
	if len(c.Body()) > 0 && c.BodyParser(&req) != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid heartbeat payload"})
	}
	const prefix = "Bearer "
	authorization := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
	if !strings.HasPrefix(authorization, prefix) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid heartbeat credentials"})
	}
	monitor, created, err := h.service.ReceiveHeartbeat(c.UserContext(), strings.TrimSpace(strings.TrimPrefix(authorization, prefix)), c.Get("Idempotency-Key"), req.Outcome)
	if errors.Is(err, application.ErrHeartbeatMonitorNotFound) || errors.Is(err, domain.ErrInvalidInput) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid heartbeat credentials"})
	}
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "heartbeat service is temporarily unavailable"})
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"monitorId": monitor.ID, "accepted": created})
}
