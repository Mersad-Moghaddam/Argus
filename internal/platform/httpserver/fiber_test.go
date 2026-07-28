package httpserver

import (
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
