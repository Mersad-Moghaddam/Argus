package httpserver

import (
	"argus/internal/api"
	"argus/internal/service"

	"github.com/gofiber/fiber/v2"
)

// NewFiberApp configures and returns the Fiber application.
func NewFiberApp(websiteService *service.WebsiteService) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "Argus Distributed Uptime Checker"})

	handler := api.NewWebsiteHandler(websiteService)
	apiGroup := app.Group("/api")
	api.RegisterWebsiteRoutes(apiGroup, handler)

	app.Static("/", "./web")
	return app
}
