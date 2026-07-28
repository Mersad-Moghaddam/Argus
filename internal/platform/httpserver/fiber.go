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
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// maxUploadBytes bounds request bodies (including multipart OpenAPI/Swagger
// spec uploads) well above the parser's own document size limit so Fiber
// rejects oversized requests before they reach handler code.
const maxUploadBytes = 15 * 1024 * 1024
const maxControlBodyBytes = 256 * 1024

func NewFiberApp(service *application.Service, logStore *observability.LogStore, apiKey string, authCookieSecure ...bool) *fiber.App {
	cookieSecure := false
	if len(authCookieSecure) > 0 {
		cookieSecure = authCookieSecure[0]
	}
	app := fiber.New(fiber.Config{
		AppName: "Argus Distributed Uptime Checker", BodyLimit: maxUploadBytes,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second,
		ReadBufferSize: 8 * 1024,
	})
	app.Use(recover.New())
	app.Use(serverTelemetry)
	app.Use(helmet.New())
	app.Use(securityHeaders)
	app.Use(etag.New())
	app.Use(compress.New(compress.Config{Level: compress.LevelBestSpeed}))

	apiGroup := app.Group("/api")
	// Authentication is intentionally bounded before bcrypt work occurs. This
	// in-process limiter is a baseline; production multi-instance deployments
	// should use a shared Fiber limiter storage at the edge or in Redis.
	apiGroup.Use("/auth", controlBodyLimit(maxControlBodyBytes), limiter.New(limiter.Config{
		Max: 100, Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "too many authentication attempts; try again later"})
		},
	}))

	// Legacy single-tenant website/heartbeat/TLS monitoring API: unchanged
	// behavior, still protected by the global X-API-Key when configured. The
	// guard is attached per route rather than as /api-wide middleware, because
	// the project API below shares the /api prefix but uses bearer tokens; a
	// group-level Use would apply the API-key check to both.
	legacyGuard := adapterhttp.APIKeyAuth(apiKey)
	websiteHandler := api.NewWebsiteHandler(service)
	logHandler := api.NewLogHandler(logStore)
	featureHandler := api.NewFeatureHandler(service)
	api.RegisterWebsiteRoutes(apiGroup, websiteHandler, legacyGuard)
	api.RegisterLogRoutes(apiGroup, logHandler, legacyGuard)
	api.RegisterFeatureRoutes(apiGroup, featureHandler, legacyGuard)

	// Project-based API route monitoring: separate bearer-token user auth,
	// independent from the legacy API key.
	authHandler := api.NewAuthHandler(service, cookieSecure)
	bearerGuard := adapterhttp.BearerAuth(service)
	api.RegisterAuthRoutes(apiGroup, authHandler, bearerGuard, adapterhttp.CSRFProtect)

	projectHandler := api.NewProjectHandler(service)
	routeHandler := api.NewRouteHandler(service)
	importHandler := api.NewImportHandler(service)
	api.RegisterProjectRoutes(apiGroup, projectHandler, bearerGuard, adapterhttp.CSRFProtect)
	api.RegisterRouteRoutes(apiGroup, routeHandler, bearerGuard, adapterhttp.CSRFProtect)
	api.RegisterImportRoutes(apiGroup, importHandler, bearerGuard, adapterhttp.CSRFProtect)

	app.Static("/", "./frontend", fiber.Static{
		Compress:      true,
		CacheDuration: 10 * time.Second,
		// Frontend assets intentionally use stable names (index.html, app.js,
		// projects.js, styles.css). Require browsers to revalidate them so a
		// deployment cannot leave users on a day-old dashboard that is missing
		// newly added tabs or behavior.
		MaxAge:         0,
		ModifyResponse: requireStaticRevalidation,
	})
	return app
}

func serverTelemetry(c *fiber.Ctx) error {
	started := time.Now()
	ctx, span := otel.Tracer("argus/http").Start(c.UserContext(), "HTTP request")
	c.SetUserContext(ctx)
	err := c.Next()
	route := c.Route().Path
	if route == "" {
		route = "/unmatched"
	}
	status := c.Response().StatusCode()
	span.SetAttributes(
		attribute.String("http.request.method", c.Method()),
		attribute.String("http.route", route),
		attribute.Int("http.response.status_code", status),
		attribute.Int64("http.server.duration_ms", time.Since(started).Milliseconds()),
	)
	if status >= 500 || err != nil {
		span.SetStatus(codes.Error, "server request failed")
	}
	span.End()
	return err
}

func controlBodyLimit(limit int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Request().Header.ContentLength() > limit {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "request payload is too large"})
		}
		return c.Next()
	}
}

func securityHeaders(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentSecurityPolicy, "default-src 'self'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
	c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
	c.Set(fiber.HeaderReferrerPolicy, "strict-origin-when-cross-origin")
	c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
	return c.Next()
}

func requireStaticRevalidation(c *fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-cache")
	return nil
}
