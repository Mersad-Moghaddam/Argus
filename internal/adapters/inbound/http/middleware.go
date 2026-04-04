package http

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func APIKeyAuth(apiKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api/public/status/") {
			return c.Next()
		}
		if apiKey == "" {
			return c.Next()
		}
		auth := strings.TrimSpace(c.Get("X-API-Key"))
		if auth != apiKey {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		return c.Next()
	}
}
