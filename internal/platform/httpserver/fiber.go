package httpserver

import (
	"argus/internal/api"
	"argus/internal/observability"
	"argus/internal/service"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

// NewFiberApp configures and returns the Fiber application.
func NewFiberApp(websiteService *service.WebsiteService, logStore *observability.LogStore) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "Argus Distributed Uptime Checker"})
	app.Use(recover.New())

	websiteHandler := api.NewWebsiteHandler(websiteService)
	logHandler := api.NewLogHandler(logStore)
	featureHandler := api.NewFeatureHandler(websiteService)

	apiGroup := app.Group("/api")
	api.RegisterWebsiteRoutes(apiGroup, websiteHandler)
	api.RegisterLogRoutes(apiGroup, logHandler)
	api.RegisterFeatureRoutes(apiGroup, featureHandler)

	app.Static("/", "./web")
	return app
}
