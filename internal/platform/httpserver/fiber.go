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

// maxUploadBytes bounds request bodies (including multipart OpenAPI/Swagger
// spec uploads) well above the parser's own document size limit so Fiber
// rejects oversized requests before they reach handler code.
const maxUploadBytes = 15 * 1024 * 1024

func NewFiberApp(service *application.Service, logStore *observability.LogStore, apiKey string) *fiber.App {
	app := fiber.New(fiber.Config{AppName: "Argus Distributed Uptime Checker", BodyLimit: maxUploadBytes})
	app.Use(recover.New())
	app.Use(helmet.New())
	app.Use(etag.New())
	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))

	apiGroup := app.Group("/api")

	// Legacy single-tenant website/heartbeat/TLS monitoring API: unchanged
	// behavior, still protected by the global X-API-Key when configured.
	legacyGroup := apiGroup.Group("", adapterhttp.APIKeyAuth(apiKey))
	websiteHandler := api.NewWebsiteHandler(service)
	logHandler := api.NewLogHandler(logStore)
	featureHandler := api.NewFeatureHandler(service)
	api.RegisterWebsiteRoutes(legacyGroup, websiteHandler)
	api.RegisterLogRoutes(legacyGroup, logHandler)
	api.RegisterFeatureRoutes(legacyGroup, featureHandler)

	// Project-based API route monitoring: separate bearer-token user auth,
	// independent from the legacy API key.
	authHandler := api.NewAuthHandler(service)
	api.RegisterAuthRoutes(apiGroup, authHandler, adapterhttp.BearerAuth(service))

	projectAPI := apiGroup.Group("", adapterhttp.BearerAuth(service))
	projectHandler := api.NewProjectHandler(service)
	routeHandler := api.NewRouteHandler(service)
	importHandler := api.NewImportHandler(service)
	api.RegisterProjectRoutes(projectAPI, projectHandler)
	api.RegisterRouteRoutes(projectAPI, routeHandler)
	api.RegisterImportRoutes(projectAPI, importHandler)

	app.Static("/", "./frontend", fiber.Static{
		Compress:      true,
		CacheDuration: 10 * time.Minute,
		MaxAge:        86400,
	})
	return app
}
