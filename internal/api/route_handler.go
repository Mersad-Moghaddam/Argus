package api

import (
	"errors"
	"strconv"
	"strings"

	"argus/internal/application"
	"argus/internal/domain"
	"argus/internal/models"

	"github.com/gofiber/fiber/v2"
)

type RouteHandler struct{ service *application.Service }

func NewRouteHandler(service *application.Service) *RouteHandler {
	return &RouteHandler{service: service}
}

func RegisterRouteRoutes(app fiber.Router, h *RouteHandler, guards ...fiber.Handler) {
	app.Post("/route/normalization/:projectId", guarded(guards, h.PreviewNormalization)...)
	app.Get("/route/catalog/:projectId", guarded(guards, h.ListRoutes)...)
	app.Post("/route/catalog/:projectId", guarded(guards, h.CreateRoute)...)
	app.Post("/route/bulk/:projectId", guarded(guards, h.BulkCreateRoutes)...)
	app.Post("/route/removal/:projectId", guarded(guards, h.BulkDeleteRoutes)...)
	app.Get("/route/catalog/:projectId/:routeId", guarded(guards, h.GetRoute)...)
	app.Put("/route/catalog/:projectId/:routeId", guarded(guards, h.UpdateRoute)...)
	app.Post("/route/enable/:projectId/:routeId", guarded(guards, h.EnableRoute)...)
	app.Post("/route/disable/:projectId/:routeId", guarded(guards, h.DisableRoute)...)
	app.Delete("/route/catalog/:projectId/:routeId", guarded(guards, h.DeleteRoute)...)
	app.Get("/route/checks/:projectId/:routeId", guarded(guards, h.ListRouteChecks)...)
	app.Get("/route/incidents/:projectId", guarded(guards, h.ListIncidents)...)
	app.Post("/route/acknowledge/:projectId/:incidentId", guarded(guards, h.AcknowledgeIncident)...)
	app.Get("/route/metrics/:projectId", guarded(guards, h.ListMetricsTimeseries)...)
}

type normalizationPreviewRequest struct {
	Method        string `json:"method"`
	BaseURL       string `json:"baseUrl"`
	RouteTemplate string `json:"routeTemplate"`
	IntervalSecs  int    `json:"intervalSeconds"`
}

// PreviewNormalization lets an editor inspect the backend-owned canonical
// endpoint identity before any catalog entry or synthetic definition is saved.
func (h *RouteHandler) PreviewNormalization(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	var req normalizationPreviewRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"code": "invalid_request", "error": "invalid request payload"})
	}
	normalized, err := domain.NormalizeEndpoint(req.Method, req.BaseURL, req.RouteTemplate)
	if err != nil {
		return normalizationErrorResponse(c, err)
	}
	duplicate := false
	if existing, lookupErr := h.service.GetRouteByMethodPath(c.UserContext(), project.ID, normalized.Method, normalized.RouteTemplate); lookupErr == nil && existing != nil {
		duplicate = true
	}
	interval := req.IntervalSecs
	if interval <= 0 {
		interval = project.DefaultIntervalSeconds
	}
	if interval <= 0 {
		interval = 300
	}
	methodClass := "unsafe"
	if domain.IsSafeSyntheticMethod(normalized.Method) {
		methodClass = "safe"
	}
	return c.JSON(fiber.Map{
		"valid": true,
		"canonical": fiber.Map{
			"method": normalized.Method, "baseUrl": normalized.BaseURL,
			"routeTemplate": normalized.RouteTemplate, "identity": normalized.CanonicalIdentity,
		},
		"fetchTarget": normalized.FetchTarget,
		"changes":     normalized.Changes,
		"duplicate":   duplicate,
		"safety": fiber.Map{
			"methodClass": methodClass, "probeDefault": "disabled",
			"networkValidation": "repeated_at_execution", "traffic": "catalog_only",
			"estimatedRequestsPerDay": 86400 / interval,
		},
	})
}

type routeRequest struct {
	Method              string   `json:"method"`
	Path                string   `json:"path"`
	BaseURL             string   `json:"baseUrl"`
	Name                string   `json:"name"`
	Summary             string   `json:"summary"`
	Description         string   `json:"description"`
	Tags                []string `json:"tags"`
	Deprecated          bool     `json:"deprecated"`
	Headers             string   `json:"headers"`
	Enabled             *bool    `json:"enabled"`
	MonitorIntervalSecs int      `json:"monitorIntervalSeconds"`
	TimeoutMS           int      `json:"timeoutMs"`
	Retries             int      `json:"retries"`
	ExpectedStatusRange string   `json:"expectedStatusRange"`
	FailureThreshold    int      `json:"failureThreshold"`
	RecoverySuccesses   int      `json:"recoverySuccesses"`
}

func toRouteInput(req routeRequest) application.RouteInput {
	return application.RouteInput{
		Method: req.Method, Path: req.Path, BaseURL: req.BaseURL, Name: req.Name, Summary: req.Summary,
		Description: req.Description, Tags: req.Tags, Deprecated: req.Deprecated, Headers: req.Headers, Enabled: req.Enabled,
		MonitorIntervalSecs: req.MonitorIntervalSecs, TimeoutMS: req.TimeoutMS, Retries: req.Retries,
		ExpectedStatusRange: req.ExpectedStatusRange, FailureThreshold: req.FailureThreshold, RecoverySuccesses: req.RecoverySuccesses,
	}
}

func sanitizeRoute(rt models.APIRoute) models.APIRoute {
	rt.Headers = application.RedactHeaders(rt.Headers)
	return rt
}

func (h *RouteHandler) ListRoutes(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	filter := models.RouteFilter{
		ProjectID: project.ID, Search: c.Query("search"), Method: strings.ToUpper(c.Query("method")),
		Status: c.Query("status"), Tag: c.Query("tag"), SortBy: c.Query("sortBy"), SortDir: c.Query("sortDir"),
		Limit: limit, Offset: offset,
	}
	if raw := c.Query("enabled"); raw != "" {
		v := raw == "true" || raw == "1"
		filter.Enabled = &v
	}
	if raw := c.Query("deprecated"); raw != "" {
		v := raw == "true" || raw == "1"
		filter.Deprecated = &v
	}
	items, total, err := h.service.ListRoutes(c.UserContext(), filter)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list routes"})
	}
	for i := range items {
		items[i] = sanitizeRoute(items[i])
	}
	return c.JSON(fiber.Map{"items": items, "total": total})
}

func (h *RouteHandler) CreateRoute(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	var req routeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	route, err := h.service.CreateRoute(c.UserContext(), project, toRouteInput(req))
	if err != nil {
		return routeErrorResponse(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(sanitizeRoute(route))
}

func (h *RouteHandler) BulkCreateRoutes(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	var req struct {
		Routes []routeRequest `json:"routes"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	if len(req.Routes) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no routes provided"})
	}
	if len(req.Routes) > 5000 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "too many routes in a single bulk request (max 5000)"})
	}
	inputs := make([]application.RouteInput, len(req.Routes))
	for i, r := range req.Routes {
		inputs[i] = toRouteInput(r)
	}
	result, err := h.service.BulkCreateRoutes(c.UserContext(), project, inputs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "bulk create failed"})
	}
	for i := range result.Created {
		result.Created[i] = sanitizeRoute(result.Created[i])
	}
	return c.Status(fiber.StatusOK).JSON(result)
}

func (h *RouteHandler) loadRoute(c *fiber.Ctx, project models.Project) (*models.APIRoute, error) {
	routeID, err := parseIDParam(c, "routeId")
	if err != nil {
		return nil, c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	route, err := h.service.GetRoute(c.UserContext(), routeID)
	if err != nil {
		return nil, c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load route"})
	}
	if route == nil || route.ProjectID != project.ID {
		return nil, c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "route not found"})
	}
	return route, nil
}

func (h *RouteHandler) GetRoute(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	route, sendErr := h.loadRoute(c, project)
	if route == nil {
		return sendErr
	}
	sanitized := sanitizeRoute(*route)
	return c.JSON(sanitized)
}

func (h *RouteHandler) UpdateRoute(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	route, sendErr := h.loadRoute(c, project)
	if route == nil {
		return sendErr
	}
	var req routeRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	updated, err := h.service.UpdateRoute(c.UserContext(), *route, toRouteInput(req))
	if err != nil {
		return routeErrorResponse(c, err)
	}
	return c.JSON(sanitizeRoute(updated))
}

func (h *RouteHandler) EnableRoute(c *fiber.Ctx) error {
	return h.setEnabled(c, true)
}
func (h *RouteHandler) DisableRoute(c *fiber.Ctx) error {
	return h.setEnabled(c, false)
}
func (h *RouteHandler) setEnabled(c *fiber.Ctx, enabled bool) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	route, sendErr := h.loadRoute(c, project)
	if route == nil {
		return sendErr
	}
	if err := h.service.SetRouteEnabled(c.UserContext(), route.ID, enabled); err != nil {
		return routeErrorResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *RouteHandler) DeleteRoute(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	route, sendErr := h.loadRoute(c, project)
	if route == nil {
		return sendErr
	}
	if err := h.service.DeleteRoute(c.UserContext(), route.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to delete route"})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *RouteHandler) BulkDeleteRoutes(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no route ids provided"})
	}
	deleted, err := h.service.BulkDeleteRoutes(c.UserContext(), project.ID, req.IDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "bulk delete failed"})
	}
	return c.JSON(fiber.Map{"deleted": deleted})
}

func (h *RouteHandler) ListRouteChecks(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	route, sendErr := h.loadRoute(c, project)
	if route == nil {
		return sendErr
	}
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	checks, err := h.service.ListRouteChecks(c.UserContext(), route.ID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list checks"})
	}
	return c.JSON(checks)
}

func (h *RouteHandler) ListIncidents(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	limit, _ := strconv.Atoi(c.Query("limit", "100"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	var routeID *int64
	if raw := c.Query("routeId"); raw != "" {
		if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
			routeID = &v
		}
	}
	incidents, err := h.service.ListRouteIncidents(c.UserContext(), project.ID, routeID, c.Query("state"), limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list incidents"})
	}
	return c.JSON(incidents)
}

func (h *RouteHandler) AcknowledgeIncident(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	incidentID, err := parseIDParam(c, "incidentId")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err = h.service.AcknowledgeRouteIncident(c.UserContext(), project.ID, incidentID, currentUserID(c)); errors.Is(err, domain.ErrProjectNotFound) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "incident not found"})
	} else if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListMetricsTimeseries serves the bucketed data behind the dashboard's
// time-range charts. Passing routeId narrows it to a single route.
func (h *RouteHandler) ListMetricsTimeseries(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	var routeID *int64
	if raw := c.Query("routeId"); raw != "" {
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || v <= 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid routeId"})
		}
		// Confirm the route belongs to this project before exposing its data.
		route, getErr := h.service.GetRoute(c.UserContext(), v)
		if getErr != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load route"})
		}
		if route == nil || route.ProjectID != project.ID {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "route not found"})
		}
		routeID = &v
	}
	series, err := h.service.ListMetricsTimeseries(c.UserContext(), project.ID, routeID, c.Query("range"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load metrics"})
	}
	return c.JSON(series)
}

func routeErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidRoute), errors.Is(err, domain.ErrDuplicateRoute), errors.Is(err, domain.ErrInvalidInput), errors.Is(err, domain.ErrUnsafeSynthetic):
		return normalizationErrorResponse(c, err)
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "route operation failed"})
	}
}

func normalizationErrorResponse(c *fiber.Ctx, err error) error {
	payload := fiber.Map{"code": domain.ValidationCode(err), "error": domain.ValidationMessage(err)}
	var validation *domain.ValidationError
	if errors.As(err, &validation) {
		payload["field"] = validation.Field
	}
	return c.Status(fiber.StatusBadRequest).JSON(payload)
}
