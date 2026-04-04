package api

import (
	"errors"
	"strconv"

	"argus/internal/models"
	"argus/internal/service"
	"github.com/gofiber/fiber/v2"
)

// WebsiteHandler handles website HTTP requests.
type WebsiteHandler struct {
	service *service.WebsiteService
}

func NewWebsiteHandler(service *service.WebsiteService) *WebsiteHandler {
	return &WebsiteHandler{service: service}
}

type createWebsiteRequest struct {
	URL                    string  `json:"url"`
	HealthCheckURL         *string `json:"healthCheckUrl"`
	CheckInterval          int     `json:"checkInterval"`
	MonitorType            string  `json:"monitorType"`
	ExpectedKeyword        *string `json:"expectedKeyword"`
	TLSExpiryThresholdDays int     `json:"tlsExpiryThresholdDays"`
	HeartbeatGraceSeconds  int     `json:"heartbeatGraceSeconds"`
	StatusPageID           *int64  `json:"statusPageId"`
}

func RegisterWebsiteRoutes(app fiber.Router, handler *WebsiteHandler) {
	app.Get("/websites", handler.ListWebsites)
	app.Post("/websites", handler.CreateWebsite)
	app.Delete("/websites/:id", handler.DeleteWebsite)
	app.Post("/websites/:id/heartbeat", handler.MarkHeartbeat)
}

func (h *WebsiteHandler) CreateWebsite(c *fiber.Ctx) error {
	var request createWebsiteRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}

	website, err := h.service.CreateWebsite(c.UserContext(), service.CreateWebsiteInput{
		URL:                    request.URL,
		HealthCheckURL:         request.HealthCheckURL,
		CheckInterval:          request.CheckInterval,
		MonitorType:            request.MonitorType,
		ExpectedKeyword:        request.ExpectedKeyword,
		TLSExpiryThresholdDays: request.TLSExpiryThresholdDays,
		HeartbeatGraceSeconds:  request.HeartbeatGraceSeconds,
		StatusPageID:           request.StatusPageID,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidURL) || errors.Is(err, service.ErrInvalidInterval) || errors.Is(err, service.ErrInvalidMonitorType) || errors.Is(err, service.ErrInvalidInput) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create website"})
	}
	return c.Status(fiber.StatusCreated).JSON(website)
}

func (h *WebsiteHandler) ListWebsites(c *fiber.Ctx) error {
	items, err := h.service.ListWebsites(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list websites"})
	}
	return c.JSON(items)
}

func (h *WebsiteHandler) DeleteWebsite(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid website id"})
	}

	if err = h.service.DeleteWebsite(c.UserContext(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete website"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *WebsiteHandler) MarkHeartbeat(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid website id"})
	}
	if err = h.service.MarkHeartbeat(c.UserContext(), id); err != nil {
		if errors.Is(err, service.ErrHeartbeatNotFound) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to mark heartbeat"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// aux types for other handlers.
type createAlertChannelRequest struct {
	Name        string `json:"name"`
	ChannelType string `json:"channelType"`
	Target      string `json:"target"`
}

type createMaintenanceWindowRequest struct {
	WebsiteID *int64  `json:"websiteId"`
	StartsAt  string  `json:"startsAt"`
	EndsAt    string  `json:"endsAt"`
	Reason    *string `json:"reason"`
}

type createStatusPageRequest struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

var _ = models.Website{}
