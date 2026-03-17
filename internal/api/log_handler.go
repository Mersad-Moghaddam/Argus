package api

import (
	"strconv"

	"argus/internal/observability"
	"github.com/gofiber/fiber/v2"
)

// LogHandler handles operational log requests.
type LogHandler struct {
	store *observability.LogStore
}

// NewLogHandler creates LogHandler.
func NewLogHandler(store *observability.LogStore) *LogHandler {
	return &LogHandler{store: store}
}

// RegisterLogRoutes sets up log routes.
func RegisterLogRoutes(app fiber.Router, handler *LogHandler) {
	app.Get("/logs", handler.ListLogs)
}

// ListLogs returns recent system and worker logs.
func (h *LogHandler) ListLogs(c *fiber.Ctx) error {
	limit := 200
	if raw := c.Query("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid limit"})
		}
		limit = value
	}

	var websiteID *int64
	if raw := c.Query("websiteId"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid websiteId"})
		}
		websiteID = &value
	}

	return c.JSON(h.store.List(limit, websiteID))
}
