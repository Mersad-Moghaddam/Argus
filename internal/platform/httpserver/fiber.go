package httpserver

import (
	"argus/internal/api"
	"argus/internal/observability"
	"argus/internal/service"

	"github.com/gofiber/fiber/v2"
)

// NewFiberApp configures and returns the Fiber application.
func NewFiberApp(websiteService *service.WebsiteService, logStore *observability.LogStore) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "Argus Distributed Uptime Checker"})

	websiteHandler := api.NewWebsiteHandler(websiteService)
	logHandler := api.NewLogHandler(logStore)

	apiGroup := app.Group("/api")
	api.RegisterWebsiteRoutes(apiGroup, websiteHandler)
	api.RegisterLogRoutes(apiGroup, logHandler)

	app.Static("/", "./web")
	return app
}
