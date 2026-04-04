package api

import (
	"strconv"
	"time"

	"argus/internal/application"
	"argus/internal/models"
	"github.com/gofiber/fiber/v2"
)

type FeatureHandler struct{ service *application.Service }

func NewFeatureHandler(service *application.Service) *FeatureHandler {
	return &FeatureHandler{service: service}
}

func RegisterFeatureRoutes(app fiber.Router, h *FeatureHandler) {
	app.Get("/incidents", h.ListIncidents)
	app.Post("/alert-channels", h.CreateAlertChannel)
	app.Post("/maintenance-windows", h.CreateMaintenanceWindow)
	app.Get("/status-pages", h.ListStatusPages)
	app.Post("/status-pages", h.CreateStatusPage)
	app.Get("/public/status/:slug", h.GetPublicStatusPage)
}

func (h *FeatureHandler) ListIncidents(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	var websiteID *int64
	if raw := c.Query("websiteId"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			websiteID = &v
		}
	}
	items, err := h.service.ListIncidents(c.UserContext(), websiteID, c.Query("state"), limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to list incidents"})
	}
	return c.JSON(items)
}
func (h *FeatureHandler) CreateAlertChannel(c *fiber.Ctx) error {
	var req createAlertChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request payload"})
	}
	id, err := h.service.CreateAlertChannel(c.UserContext(), models.AlertChannel{Name: req.Name, ChannelType: req.ChannelType, Target: req.Target, Enabled: true})
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(fiber.Map{"id": id})
}
func (h *FeatureHandler) CreateMaintenanceWindow(c *fiber.Ctx) error {
	var req createMaintenanceWindowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request payload"})
	}
	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid startsAt"})
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid endsAt"})
	}
	id, err := h.service.CreateMaintenanceWindow(c.UserContext(), models.MaintenanceWindow{WebsiteID: req.WebsiteID, StartsAt: startsAt.UTC(), EndsAt: endsAt.UTC(), MuteAlerts: true, Reason: req.Reason})
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(fiber.Map{"id": id})
}
func (h *FeatureHandler) ListStatusPages(c *fiber.Ctx) error {
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	items, err := h.service.ListStatusPages(c.UserContext(), limit, offset)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to list status pages"})
	}
	return c.JSON(items)
}
func (h *FeatureHandler) CreateStatusPage(c *fiber.Ctx) error {
	var req createStatusPageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid request payload"})
	}
	id, err := h.service.CreateStatusPage(c.UserContext(), models.StatusPage{Slug: req.Slug, Title: req.Title, IsPublic: true})
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(201).JSON(fiber.Map{"id": id})
}
func (h *FeatureHandler) GetPublicStatusPage(c *fiber.Ctx) error {
	page, websites, err := h.service.GetStatusPage(c.UserContext(), c.Params("slug"))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to get status page"})
	}
	if page == nil {
		return c.Status(404).JSON(fiber.Map{"error": "status page not found"})
	}
	return c.JSON(fiber.Map{"page": page, "websites": websites})
}
