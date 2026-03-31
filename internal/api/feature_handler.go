package api

import (
	"strconv"
	"time"

	"argus/internal/models"
	"argus/internal/service"
	"github.com/gofiber/fiber/v2"
)

// FeatureHandler groups new feature endpoints.
type FeatureHandler struct {
	service *service.WebsiteService
}

func NewFeatureHandler(service *service.WebsiteService) *FeatureHandler {
	return &FeatureHandler{service: service}
}

func RegisterFeatureRoutes(app fiber.Router, handler *FeatureHandler) {
	app.Get("/incidents", handler.ListIncidents)
	app.Post("/alert-channels", handler.CreateAlertChannel)
	app.Post("/maintenance-windows", handler.CreateMaintenanceWindow)
	app.Get("/status-pages", handler.ListStatusPages)
	app.Post("/status-pages", handler.CreateStatusPage)
	app.Get("/public/status/:slug", handler.GetPublicStatusPage)
}

func (h *FeatureHandler) ListIncidents(c *fiber.Ctx) error {
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err == nil {
			limit = value
		}
	}
	var websiteID *int64
	if raw := c.Query("websiteId"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			websiteID = &v
		}
	}
	items, err := h.service.ListIncidents(c.UserContext(), websiteID, c.Query("state"), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list incidents"})
	}
	return c.JSON(items)
}

func (h *FeatureHandler) CreateAlertChannel(c *fiber.Ctx) error {
	var req createAlertChannelRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	id, err := h.service.CreateAlertChannel(c.UserContext(), models.AlertChannel{Name: req.Name, ChannelType: req.ChannelType, Target: req.Target})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (h *FeatureHandler) CreateMaintenanceWindow(c *fiber.Ctx) error {
	var req createMaintenanceWindowRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid startsAt"})
	}
	endsAt, err := time.Parse(time.RFC3339, req.EndsAt)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid endsAt"})
	}
	id, err := h.service.CreateMaintenanceWindow(c.UserContext(), models.MaintenanceWindow{WebsiteID: req.WebsiteID, StartsAt: startsAt.UTC(), EndsAt: endsAt.UTC(), Reason: req.Reason})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (h *FeatureHandler) ListStatusPages(c *fiber.Ctx) error {
	items, err := h.service.ListStatusPages(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list status pages"})
	}
	return c.JSON(items)
}

func (h *FeatureHandler) CreateStatusPage(c *fiber.Ctx) error {
	var req createStatusPageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	id, err := h.service.CreateStatusPage(c.UserContext(), models.StatusPage{Slug: req.Slug, Title: req.Title, IsPublic: true})
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": id})
}

func (h *FeatureHandler) GetPublicStatusPage(c *fiber.Ctx) error {
	page, websites, err := h.service.GetStatusPage(c.UserContext(), c.Params("slug"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to get status page"})
	}
	if page == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "status page not found"})
	}
	return c.JSON(fiber.Map{"page": page, "websites": websites})
}
