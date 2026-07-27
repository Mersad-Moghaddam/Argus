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

func RegisterRouteRoutes(app fiber.Router, h *RouteHandler) {
	app.Get("/projects/:projectId/routes", h.ListRoutes)
	app.Post("/projects/:projectId/routes", h.CreateRoute)
	app.Post("/projects/:projectId/routes/bulk", h.BulkCreateRoutes)
	app.Post("/projects/:projectId/routes/bulk-delete", h.BulkDeleteRoutes)
	app.Get("/projects/:projectId/routes/:routeId", h.GetRoute)
	app.Put("/projects/:projectId/routes/:routeId", h.UpdateRoute)
	app.Post("/projects/:projectId/routes/:routeId/enable", h.EnableRoute)
	app.Post("/projects/:projectId/routes/:routeId/disable", h.DisableRoute)
	app.Delete("/projects/:projectId/routes/:routeId", h.DeleteRoute)
	app.Get("/projects/:projectId/routes/:routeId/checks", h.ListRouteChecks)
	app.Get("/projects/:projectId/incidents", h.ListIncidents)
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
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update route"})
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

func routeErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrInvalidRoute), errors.Is(err, domain.ErrDuplicateRoute), errors.Is(err, domain.ErrInvalidInput):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "route operation failed"})
	}
}
