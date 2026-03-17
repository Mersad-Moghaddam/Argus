package api

import (
	"errors"
	"strconv"

	"argus/internal/service"
	"github.com/gofiber/fiber/v2"
)

// WebsiteHandler handles website HTTP requests.
type WebsiteHandler struct {
	service *service.WebsiteService
}

// NewWebsiteHandler creates WebsiteHandler.
func NewWebsiteHandler(service *service.WebsiteService) *WebsiteHandler {
	return &WebsiteHandler{service: service}
}

type createWebsiteRequest struct {
	URL            string  `json:"url"`
	HealthCheckURL *string `json:"healthCheckUrl"`
	CheckInterval  int     `json:"checkInterval"`
}

// RegisterWebsiteRoutes sets up website routes.
func RegisterWebsiteRoutes(app fiber.Router, handler *WebsiteHandler) {
	app.Get("/websites", handler.ListWebsites)
	app.Post("/websites", handler.CreateWebsite)
	app.Delete("/websites/:id", handler.DeleteWebsite)
}

// CreateWebsite creates website endpoint.
func (h *WebsiteHandler) CreateWebsite(c *fiber.Ctx) error {
	var request createWebsiteRequest
	if err := c.BodyParser(&request); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}

	website, err := h.service.CreateWebsite(c.UserContext(), request.URL, request.CheckInterval, request.HealthCheckURL)
	if err != nil {
		if errors.Is(err, service.ErrInvalidURL) || errors.Is(err, service.ErrInvalidInterval) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create website"})
	}
	return c.Status(fiber.StatusCreated).JSON(website)
}

// ListWebsites lists websites endpoint.
func (h *WebsiteHandler) ListWebsites(c *fiber.Ctx) error {
	items, err := h.service.ListWebsites(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list websites"})
	}
	return c.JSON(items)
}

// DeleteWebsite deletes website endpoint.
func (h *WebsiteHandler) DeleteWebsite(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid website id"})
	}

	err = h.service.DeleteWebsite(c.UserContext(), id)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete website"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}
