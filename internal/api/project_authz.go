package api

import (
	"errors"
	"strconv"

	adapterhttp "argus/internal/adapters/inbound/http"
	"argus/internal/application"
	"argus/internal/domain"
	"argus/internal/models"

	"github.com/gofiber/fiber/v2"
)

func currentUserID(c *fiber.Ctx) int64 {
	id, _ := c.Locals(adapterhttp.UserContextKey).(int64)
	return id
}

func parseIDParam(c *fiber.Ctx, name string) (int64, error) {
	v, err := strconv.ParseInt(c.Params(name), 10, 64)
	if err != nil || v <= 0 {
		return 0, errors.New("invalid " + name)
	}
	return v, nil
}

// authorizeProject centralizes project-scoped access control for every
// handler below: it resolves the project ID path param, requires the
// caller to be an authenticated member with at least minRole, and returns a
// ready-to-send fiber error for the caller to `return` directly on failure.
// authorizeProject returns (project, true) on success. On failure it writes
// the appropriate error response itself and returns (zero-value, false); the
// caller must check the bool and simply `return nil` without doing any
// further work.
func authorizeProject(c *fiber.Ctx, service *application.Service, minRole string) (models.Project, bool) {
	projectID, err := parseIDParam(c, "projectId")
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		return models.Project{}, false
	}
	userID := currentUserID(c)
	project, _, err := service.AuthorizeProject(c.UserContext(), projectID, userID, minRole)
	if err != nil {
		_ = projectErrorResponse(c, err)
		return models.Project{}, false
	}
	return *project, true
}

func projectErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, domain.ErrProjectNotFound):
		// Not-a-member and not-found are indistinguishable to the caller by
		// design, to avoid leaking which project IDs exist.
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "project not found"})
	case errors.Is(err, application.ErrInsufficientRole):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to authorize project access"})
	}
}
