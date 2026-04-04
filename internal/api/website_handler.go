package api

import (
	"errors"
	"strconv"

	"argus/internal/application"
	"argus/internal/domain"
	"argus/internal/models"
	"github.com/gofiber/fiber/v2"
)

type WebsiteHandler struct{ service *application.Service }

func NewWebsiteHandler(service *application.Service) *WebsiteHandler {
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

func RegisterWebsiteRoutes(app fiber.Router, h *WebsiteHandler) {
	app.Get("/websites", h.ListWebsites)
	app.Post("/websites", h.CreateWebsite)
	app.Delete("/websites/:id", h.DeleteWebsite)
	app.Post("/websites/:id/heartbeat", h.MarkHeartbeat)
}

func (h *WebsiteHandler) CreateWebsite(c *fiber.Ctx) error {
	var req createWebsiteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	website, err := h.service.CreateMonitor(c.UserContext(), application.CreateMonitorInput{URL: req.URL, HealthCheckURL: req.HealthCheckURL, CheckInterval: req.CheckInterval, MonitorType: req.MonitorType, ExpectedKeyword: req.ExpectedKeyword, TLSExpiryThresholdDays: req.TLSExpiryThresholdDays, HeartbeatGraceSeconds: req.HeartbeatGraceSeconds, StatusPageID: req.StatusPageID})
	if err != nil {
		if errors.Is(err, domain.ErrInvalidURL) || errors.Is(err, domain.ErrInvalidInterval) || errors.Is(err, domain.ErrInvalidMonitorType) || errors.Is(err, domain.ErrInvalidInput) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create website"})
	}
	return c.Status(fiber.StatusCreated).JSON(website)
}
func (h *WebsiteHandler) ListWebsites(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	items, err := h.service.ListMonitors(c.UserContext(), limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to list websites"})
	}
	return c.JSON(items)
}
func (h *WebsiteHandler) DeleteWebsite(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "invalid website id"})
	}
	if err = h.service.DeleteMonitor(c.UserContext(), id); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to delete website"})
	}
	return c.SendStatus(204)
}
func (h *WebsiteHandler) MarkHeartbeat(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.Status(400).JSON(fiber.Map{"error": "invalid website id"})
	}
	if err = h.service.MarkHeartbeat(c.UserContext(), id); err != nil {
		if errors.Is(err, application.ErrHeartbeatNotFound) {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(500).JSON(fiber.Map{"error": "failed to mark heartbeat"})
	}
	return c.SendStatus(204)
}

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
