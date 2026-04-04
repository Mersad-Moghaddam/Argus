package httpserver

import (
	adapterhttp "argus/internal/adapters/inbound/http"
	"argus/internal/api"
	"argus/internal/application"
	"argus/internal/observability"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func NewFiberApp(service *application.Service, logStore *observability.LogStore, apiKey string) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "Argus Distributed Uptime Checker"})
	app.Use(recover.New())
	app.Use("/api", adapterhttp.APIKeyAuth(apiKey))
	websiteHandler := api.NewWebsiteHandler(service)
	logHandler := api.NewLogHandler(logStore)
	featureHandler := api.NewFeatureHandler(service)
	apiGroup := app.Group("/api")
	api.RegisterWebsiteRoutes(apiGroup, websiteHandler)
	api.RegisterLogRoutes(apiGroup, logHandler)
	api.RegisterFeatureRoutes(apiGroup, featureHandler)
	app.Static("/", "./web")
	return app
}
