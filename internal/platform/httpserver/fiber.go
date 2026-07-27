package httpserver

import (
	"time"

	adapterhttp "argus/internal/adapters/inbound/http"
	"argus/internal/api"
	"argus/internal/application"
	"argus/internal/observability"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/etag"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func NewFiberApp(service *application.Service, logStore *observability.LogStore, apiKey string) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "Argus Distributed Uptime Checker"})
	app.Use(recover.New())
	app.Use(helmet.New())
	app.Use(etag.New())
	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))
	app.Use("/api", adapterhttp.APIKeyAuth(apiKey))
	websiteHandler := api.NewWebsiteHandler(service)
	logHandler := api.NewLogHandler(logStore)
	featureHandler := api.NewFeatureHandler(service)
	apiGroup := app.Group("/api")
	api.RegisterWebsiteRoutes(apiGroup, websiteHandler)
	api.RegisterLogRoutes(apiGroup, logHandler)
	api.RegisterFeatureRoutes(apiGroup, featureHandler)
	app.Static("/", "./frontend", fiber.Static{
		Compress:      true,
		CacheDuration: 10 * time.Minute,
		MaxAge:        86400,
	})
	return app
}
