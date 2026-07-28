package httpserver

import (
	"bytes"
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
