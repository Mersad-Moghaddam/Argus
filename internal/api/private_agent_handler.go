package api

import (
	"argus/internal/application"
	"argus/internal/models"
	"errors"
	"github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
)

type PrivateAgentHandler struct{ service *application.Service }

func NewPrivateAgentHandler(s *application.Service) *PrivateAgentHandler {
	return &PrivateAgentHandler{service: s}
}

type agentHeartbeatRequest struct {
	Version string `json:"version"`
}
type agentResultRequest struct {
	Outcome string `json:"outcome"`
	Summary string `json:"summary"`
	Version string `json:"version"`
}

type privateAgentRequest struct {
	Name                    string `json:"name"`
	EnvironmentID           int64  `json:"environmentId"`
	ExpectedIntervalSeconds int    `json:"expectedIntervalSeconds"`
}

func (h *PrivateAgentHandler) Heartbeat(c *fiber.Ctx) error {
	var req agentHeartbeatRequest
	if len(c.Body()) > 0 && c.BodyParser(&req) != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid agent payload"})
	}
	auth := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
	const p = "Bearer "
	if !strings.HasPrefix(auth, p) {
		return c.Status(401).JSON(fiber.Map{"error": "invalid agent credentials"})
	}
	agent, err := h.service.AuthenticatePrivateAgent(c.UserContext(), strings.TrimSpace(strings.TrimPrefix(auth, p)), req.Version)
	if errors.Is(err, application.ErrPrivateAgentNotFound) {
		return c.Status(401).JSON(fiber.Map{"error": "invalid agent credentials"})
	}
	if err != nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent service unavailable"})
	}
	return c.JSON(fiber.Map{"agentId": agent.ID, "projectId": agent.ProjectID, "environmentId": agent.EnvironmentID})
}

func (h *PrivateAgentHandler) Configuration(c *fiber.Ctx) error {
	auth := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
	const p = "Bearer "
	if !strings.HasPrefix(auth, p) {
		return c.Status(401).JSON(fiber.Map{"error": "invalid agent credentials"})
	}
	signed, err := h.service.IssuePrivateAgentConfiguration(c.UserContext(), strings.TrimSpace(strings.TrimPrefix(auth, p)), c.Get("Argus-Agent-Version"))
	if errors.Is(err, application.ErrPrivateAgentNotFound) {
		return c.Status(401).JSON(fiber.Map{"error": "invalid agent credentials"})
	}
	if err != nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent configuration unavailable"})
	}
	return c.JSON(signed)
}
func (h *PrivateAgentHandler) Result(c *fiber.Ctx) error {
	var req agentResultRequest
	if c.BodyParser(&req) != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid agent result"})
	}
	auth := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
	const p = "Bearer "
	if !strings.HasPrefix(auth, p) {
		return c.Status(401).JSON(fiber.Map{"error": "invalid agent credentials"})
	}
	created, err := h.service.RecordPrivateAgentResult(c.UserContext(), strings.TrimSpace(strings.TrimPrefix(auth, p)), req.Version, c.Get("Idempotency-Key"), req.Outcome, req.Summary)
	if errors.Is(err, application.ErrPrivateAgentNotFound) {
		return c.Status(401).JSON(fiber.Map{"error": "invalid agent credentials"})
	}
	if errors.Is(err, application.ErrInvalidPrivateAgentResult) {
		return c.Status(400).JSON(fiber.Map{"error": "invalid agent result"})
	}
	if err != nil {
		return c.Status(503).JSON(fiber.Map{"error": "agent result service unavailable"})
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"accepted": created})
}

func (h *PrivateAgentHandler) List(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	items, err := h.service.ListPrivateAgents(c.UserContext(), project.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list private agents"})
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *PrivateAgentHandler) Create(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	var req privateAgentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	issued, err := h.service.CreatePrivateAgent(c.UserContext(), project.ID, currentUserID(c), application.CreatePrivateAgentInput{Name: req.Name, EnvironmentID: req.EnvironmentID, ExpectedIntervalSeconds: req.ExpectedIntervalSeconds})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(issued)
}

func (h *PrivateAgentHandler) Revoke(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	agentID, err := strconv.ParseInt(c.Params("agentId"), 10, 64)
	if err != nil || agentID <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid agent id"})
	}
	if err = h.service.RevokePrivateAgent(c.UserContext(), project.ID, agentID); errors.Is(err, application.ErrPrivateAgentNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "private agent not found"})
	} else if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to revoke private agent"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func RegisterPrivateAgentRoutes(app fiber.Router, h *PrivateAgentHandler) {
	app.Post("/agent/heartbeat", h.Heartbeat)
	app.Get("/agent/config", h.Configuration)
	app.Post("/agent/result", h.Result)
}

func RegisterPrivateAgentManagementRoutes(app fiber.Router, h *PrivateAgentHandler, guards ...fiber.Handler) {
	app.Get("/agent/catalog/:projectId", guarded(guards, h.List)...)
	app.Post("/agent/catalog/:projectId", guarded(guards, h.Create)...)
	app.Post("/agent/revoke/:projectId/:agentId", guarded(guards, h.Revoke)...)
}
