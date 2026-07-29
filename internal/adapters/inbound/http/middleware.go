package http

import (
	"crypto/subtle"
	"strings"

	"argus/internal/application"

	"github.com/gofiber/fiber/v2"
)

func APIKeyAuth(apiKey string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if apiKey == "" {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "legacy API is disabled until API_KEY is configured"})
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

// UserContextUserKey contains the authenticated account for handlers that
// need to return account information without performing another lookup.
const UserContextUserKey = "user"

const (
	// SessionCookieName deliberately has no Domain attribute, keeping it scoped
	// to the host that issued it. It is HttpOnly when created by AuthHandler.
	SessionCookieName = "argus_session"
	CSRFCookieName    = "argus_csrf"
	CSRFHeaderName    = "X-CSRF-Token"
	authSchemeCookie  = "cookie"
	authSchemeBearer  = "bearer"
	authSchemeKey     = "argus.auth.scheme"
)

// BearerAuth authenticates project-API requests using an opaque bearer
// token issued at register/login. It is independent from APIKeyAuth so the
// existing single-tenant website-monitoring API keeps working unchanged.
func BearerAuth(service *application.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := strings.TrimSpace(c.Get("Authorization"))
		token, scheme := "", ""
		if strings.HasPrefix(header, "Bearer ") {
			token, scheme = strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")), authSchemeBearer
		} else if header == "" {
			token, scheme = strings.TrimSpace(c.Cookies(SessionCookieName)), authSchemeCookie
		}
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
		}
		user, err := service.Authenticate(c.UserContext(), token)
		if err != nil || user == nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired session"})
		}
		c.Locals(UserContextKey, user.ID)
		c.Locals(UserContextUserKey, *user)
		c.Locals(authSchemeKey, scheme)
		return c.Next()
	}
}

// CSRFProtect requires a matching readable CSRF cookie and request header for
// unsafe requests authenticated with a browser cookie. Bearer automation
// tokens do not rely on ambient browser credentials and are therefore exempt.
func CSRFProtect(c *fiber.Ctx) error {
	switch c.Method() {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
		return c.Next()
	}
	if c.Locals(authSchemeKey) != authSchemeCookie {
		return c.Next()
	}
	cookie := c.Cookies(CSRFCookieName)
	header := strings.TrimSpace(c.Get(CSRFHeaderName))
	if cookie == "" || header == "" || subtle.ConstantTimeCompare([]byte(cookie), []byte(header)) != 1 {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "invalid CSRF token"})
	}
	return c.Next()
}
