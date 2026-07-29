package httpserver

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRequireStaticRevalidation(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		if err := requireStaticRevalidation(c); err != nil {
			return err
		}
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(fiber.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("request frontend asset: %v", err)
	}
	defer resp.Body.Close()

	if got, want := resp.Header.Get(fiber.HeaderCacheControl), "no-cache"; got != want {
		t.Fatalf("Cache-Control = %q, want %q", got, want)
	}
}

func TestFrontendHistoryFallbackServesHTMLNavigationsOnly(t *testing.T) {
	app := fiber.New()
	app.Get("/identity/profile", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusUnauthorized) })
	registerFrontendHistoryRoutes(app, func(c *fiber.Ctx) error { return c.SendString("dashboard shell") })

	for _, path := range []string{"/account", "/login", "/projects", "/projects/42", "/projects/42/routes/7", "/future-client-route"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(fiber.HeaderAccept, "text/html,application/xhtml+xml")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request %s: %v", path, err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("HTML navigation %s returned %d, want %d", path, resp.StatusCode, fiber.StatusOK)
		}
		_ = resp.Body.Close()
	}

	apiReq := httptest.NewRequest(http.MethodGet, "/not-an-api-route", nil)
	apiReq.Header.Set(fiber.HeaderAccept, "application/json")
	apiResp, err := app.Test(apiReq)
	if err != nil {
		t.Fatalf("API request: %v", err)
	}
	defer apiResp.Body.Close()
	if apiResp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("API request returned %d, want %d", apiResp.StatusCode, fiber.StatusNotFound)
	}

	protectedReq := httptest.NewRequest(http.MethodGet, "/identity/profile", nil)
	protectedReq.Header.Set(fiber.HeaderAccept, "text/html")
	protectedResp, err := app.Test(protectedReq)
	if err != nil {
		t.Fatalf("protected request: %v", err)
	}
	defer protectedResp.Body.Close()
	if protectedResp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("registered API route returned %d, want %d", protectedResp.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestSecurityHeadersAndControlBodyLimit(t *testing.T) {
	app := fiber.New()
	app.Use(securityHeaders)
	app.Post("/control", controlBodyLimit(8), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	small, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/control", bytes.NewBufferString("small")))
	if err != nil {
		t.Fatalf("small request: %v", err)
	}
	if small.StatusCode != fiber.StatusNoContent {
		t.Fatalf("small status = %d", small.StatusCode)
	}
	if got := small.Header.Get(fiber.HeaderContentSecurityPolicy); got == "" {
		t.Fatal("missing Content-Security-Policy")
	}
	if got := small.Header.Get("Permissions-Policy"); got == "" {
		t.Fatal("missing Permissions-Policy")
	}
	_ = small.Body.Close()

	large, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/control", bytes.NewBufferString("this is too large")))
	if err != nil {
		t.Fatalf("large request: %v", err)
	}
	defer large.Body.Close()
	if large.StatusCode != fiber.StatusRequestEntityTooLarge {
		t.Fatalf("large status = %d, want 413", large.StatusCode)
	}
}
