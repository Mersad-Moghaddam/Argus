package http

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAPIKeyAuthFailsClosedWhenKeyIsUnset(t *testing.T) {
	app := fiber.New()
	app.Post("/legacy", APIKeyAuth(""), func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	response, err := app.Test(httptest.NewRequest(fiber.MethodPost, "/legacy", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusServiceUnavailable {
		t.Fatalf("unset key status = %d, want %d", response.StatusCode, fiber.StatusServiceUnavailable)
	}
}
