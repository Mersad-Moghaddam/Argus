package http

import (
	"strings"

	"argus/internal/application"

	"github.com/gofiber/fiber/v2"
)

func APIKeyAuth(apiKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
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

// UserContextKey is the fiber.Locals key holding the authenticated user's ID
// for project-scoped API routes.
const UserContextKey = "userID"

// BearerAuth authenticates project-API requests using an opaque bearer
// token issued at register/login. It is independent from APIKeyAuth so the
// existing single-tenant website-monitoring API keeps working unchanged.
func BearerAuth(service *application.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := strings.TrimSpace(c.Get("Authorization"))
		token := strings.TrimPrefix(header, "Bearer ")
		if token == "" || token == header {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing or invalid authorization header"})
		}
		user, err := service.Authenticate(c.UserContext(), token)
		if err != nil || user == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired session"})
		}
		c.Locals(UserContextKey, user.ID)
		return c.Next()
	}
}
