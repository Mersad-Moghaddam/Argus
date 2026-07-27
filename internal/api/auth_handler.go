package api

import (
	"errors"

	adapterhttp "argus/internal/adapters/inbound/http"
	"argus/internal/application"

	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct{ service *application.Service }

func NewAuthHandler(service *application.Service) *AuthHandler { return &AuthHandler{service: service} }

func RegisterAuthRoutes(app fiber.Router, h *AuthHandler, authed fiber.Handler) {
	app.Post("/auth/register", h.Register)
	app.Post("/auth/login", h.Login)
	app.Post("/auth/logout", authed, h.Logout)
	app.Get("/auth/me", authed, h.Me)
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	result, err := h.service.Register(c.UserContext(), req.Email, req.Password, req.Name)
	if err != nil {
		return authErrorResponse(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"user": result.User, "token": result.Token})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	result, err := h.service.Login(c.UserContext(), req.Email, req.Password)
	if err != nil {
		return authErrorResponse(c, err)
	}
	return c.JSON(fiber.Map{"user": result.User, "token": result.Token})
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	header := c.Get("Authorization")
	token := stripBearer(header)
	if err := h.service.Logout(c.UserContext(), token); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to logout"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID, _ := c.Locals(adapterhttp.UserContextKey).(int64)
	return c.JSON(fiber.Map{"userId": userID})
}

func stripBearer(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):]
	}
	return header
}

func authErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, application.ErrEmailTaken), errors.Is(err, application.ErrInvalidEmail), errors.Is(err, application.ErrWeakPassword):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, application.ErrInvalidCredentials):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "authentication failed"})
	}
}
