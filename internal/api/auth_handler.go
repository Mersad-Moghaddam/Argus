package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	adapterhttp "argus/internal/adapters/inbound/http"
	"argus/internal/application"
	"argus/internal/models"

	"github.com/gofiber/fiber/v2"
)

const sessionTTL = 30 * 24 * time.Hour

type AuthHandler struct {
	service      *application.Service
	cookieSecure bool
}

func NewAuthHandler(service *application.Service, cookieSecure ...bool) *AuthHandler {
	secure := false
	if len(cookieSecure) > 0 {
		secure = cookieSecure[0]
	}
	return &AuthHandler{service: service, cookieSecure: secure}
}

func RegisterAuthRoutes(app fiber.Router, h *AuthHandler, guards ...fiber.Handler) {
	app.Post("/identity/register", h.Register)
	app.Post("/identity/login", h.Login)
	app.Post("/identity/recovery/request", h.RequestPasswordRecovery)
	app.Post("/identity/recovery/complete", h.CompletePasswordRecovery)
	app.Post("/identity/logout", guarded(guards, h.Logout)...)
	app.Get("/identity/profile", guarded(guards[:1], h.Me)...)
	app.Get("/identity/sessions", guarded(guards[:1], h.ListSessions)...)
	app.Post("/identity/sessions/revoke-others", guarded(guards, h.RevokeOtherSessions)...)
	app.Post("/identity/password", guarded(guards, h.ChangePassword)...)
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
type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}
type passwordRecoveryRequest struct {
	Email string `json:"email"`
}
type passwordRecoveryCompleteRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"newPassword"`
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
	if err := h.setSession(c, result.Token); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not start session"})
	}
	// Token remains in this response during the documented bearer-token
	// compatibility window for non-browser automation. The browser client uses
	// only the HttpOnly cookie set above and never persists this field.
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
	if err := h.setSession(c, result.Token); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not start session"})
	}
	return c.JSON(fiber.Map{"user": result.User, "token": result.Token})
}

// RequestPasswordRecovery always returns the same accepted response. This
// prevents callers from learning whether an email is registered or whether
// the optional delivery integration is configured.
func (h *AuthHandler) RequestPasswordRecovery(c *fiber.Ctx) error {
	var req passwordRecoveryRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	_ = h.service.RequestPasswordRecovery(c.UserContext(), req.Email)
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"message": "If an account is eligible, recovery instructions have been sent."})
}

func (h *AuthHandler) CompletePasswordRecovery(c *fiber.Ctx) error {
	var req passwordRecoveryCompleteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	if err := h.service.CompletePasswordRecovery(c.UserContext(), req.Token, req.NewPassword); err != nil {
		if errors.Is(err, application.ErrWeakPassword) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid or expired recovery token"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	token := currentRawToken(c)
	if err := h.service.Logout(c.UserContext(), token); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to logout"})
	}
	h.clearSession(c)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *AuthHandler) ListSessions(c *fiber.Ctx) error {
	userID, _ := c.Locals(adapterhttp.UserContextKey).(int64)
	sessions, err := h.service.ListSessions(c.UserContext(), userID, currentRawToken(c))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list sessions"})
	}
	return c.JSON(fiber.Map{"sessions": sessions})
}

func (h *AuthHandler) RevokeOtherSessions(c *fiber.Ctx) error {
	userID, _ := c.Locals(adapterhttp.UserContextKey).(int64)
	if err := h.service.RevokeOtherSessions(c.UserContext(), userID, currentRawToken(c)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired session"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	var req changePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	userID, _ := c.Locals(adapterhttp.UserContextKey).(int64)
	if err := h.service.ChangePassword(c.UserContext(), userID, currentRawToken(c), req.CurrentPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, application.ErrWeakPassword), errors.Is(err, application.ErrCurrentPassword):
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		case errors.Is(err, application.ErrInvalidToken):
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid or expired session"})
		default:
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "password could not be changed"})
		}
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *AuthHandler) Me(c *fiber.Ctx) error {
	user, _ := c.Locals(adapterhttp.UserContextUserKey).(models.User)
	return c.JSON(fiber.Map{"user": user})
}

func (h *AuthHandler) setSession(c *fiber.Ctx, token string) error {
	csrf, err := randomToken()
	if err != nil {
		return err
	}
	maxAge := int(sessionTTL.Seconds())
	c.Cookie(&fiber.Cookie{Name: adapterhttp.SessionCookieName, Value: token, Path: "/", MaxAge: maxAge, HTTPOnly: true, Secure: h.cookieSecure, SameSite: "Lax"})
	c.Cookie(&fiber.Cookie{Name: adapterhttp.CSRFCookieName, Value: csrf, Path: "/", MaxAge: maxAge, HTTPOnly: false, Secure: h.cookieSecure, SameSite: "Lax"})
	return nil
}

func (h *AuthHandler) clearSession(c *fiber.Ctx) {
	for _, name := range []string{adapterhttp.SessionCookieName, adapterhttp.CSRFCookieName} {
		c.Cookie(&fiber.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HTTPOnly: name == adapterhttp.SessionCookieName, Secure: h.cookieSecure, SameSite: "Lax"})
	}
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func stripBearer(header string) string {
	const prefix = "Bearer "
	if len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):]
	}
	return header
}

func currentRawToken(c *fiber.Ctx) string {
	token := stripBearer(c.Get("Authorization"))
	if token == "" {
		token = c.Cookies(adapterhttp.SessionCookieName)
	}
	return token
}

func authErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, application.ErrInvalidEmail), errors.Is(err, application.ErrWeakPassword):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, application.ErrEmailTaken):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "registration could not be completed"})
	case errors.Is(err, application.ErrInvalidCredentials):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "authentication failed"})
	}
}
