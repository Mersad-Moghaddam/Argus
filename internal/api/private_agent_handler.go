package api

import (
	"argus/internal/application"
	"errors"
	"github.com/gofiber/fiber/v2"
	"strings"
)

type PrivateAgentHandler struct{ service *application.Service }

func NewPrivateAgentHandler(s *application.Service) *PrivateAgentHandler {
	return &PrivateAgentHandler{service: s}
}

type agentHeartbeatRequest struct {
	Version string `json:"version"`
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
func RegisterPrivateAgentRoutes(app fiber.Router, h *PrivateAgentHandler) {
	app.Post("/agent/heartbeat", h.Heartbeat)
}
