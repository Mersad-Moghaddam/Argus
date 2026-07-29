package api

import (
	"errors"
	"io"

	"argus/internal/application"
	"argus/internal/domain"
	"argus/internal/models"
	"argus/internal/openapi"

	"github.com/gofiber/fiber/v2"
)

type ImportHandler struct{ service *application.Service }

// MaxImportCommitBodyBytes bounds only the explicit selection list. Validation
// uploads retain the larger application limit needed for a 10 MiB spec.
const MaxImportCommitBodyBytes = 1024 * 1024

func NewImportHandler(service *application.Service) *ImportHandler {
	return &ImportHandler{service: service}
}

func RegisterImportRoutes(app fiber.Router, h *ImportHandler, guards ...fiber.Handler) {
	app.Post("/import/validation/:projectId", guarded(guards, h.Validate)...)
	app.Get("/import/job/:projectId/:jobId", guarded(guards, h.GetJob)...)
	commitGuards := make([]fiber.Handler, 0, len(guards)+1)
	commitGuards = append(commitGuards, importCommitBodyLimit)
	commitGuards = append(commitGuards, guards...)
	app.Post("/import/commit/:projectId/:jobId", guarded(commitGuards, h.Commit)...)
}

func importCommitBodyLimit(c *fiber.Ctx) error {
	if c.Request().Header.ContentLength() > MaxImportCommitBodyBytes || len(c.Body()) > MaxImportCommitBodyBytes {
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "import selection payload is too large"})
	}
	return c.Next()
}

type pasteImportRequest struct {
	Spec            string `json:"spec"`
	BaseURLOverride string `json:"baseUrlOverride"`
}

// Validate accepts either a multipart file upload (field "file") or a JSON
// body with a pasted spec string, parses it, diffs it against the project's
// existing routes, and returns a preview the caller must explicitly commit.
func (h *ImportHandler) Validate(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}

	var data []byte
	sourceType := models.ImportSourcePaste
	baseURLOverride := ""

	if fileHeader, err := c.FormFile("file"); err == nil {
		sourceType = models.ImportSourceFile
		baseURLOverride = c.FormValue("baseUrlOverride")
		if fileHeader.Size > openapi.MaxDocumentBytes {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": "uploaded file exceeds the maximum allowed size"})
		}
		f, openErr := fileHeader.Open()
		if openErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unable to read uploaded file"})
		}
		defer f.Close()
		data, err = io.ReadAll(io.LimitReader(f, openapi.MaxDocumentBytes+1))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unable to read uploaded file"})
		}
	} else {
		var req pasteImportRequest
		if parseErr := c.BodyParser(&req); parseErr != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
		}
		data = []byte(req.Spec)
		baseURLOverride = req.BaseURLOverride
	}

	if len(data) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no specification content provided"})
	}

	job, err := h.service.ValidateImport(c.UserContext(), application.ValidateImportInput{
		ProjectID: project.ID, UserID: currentUserID(c), SourceType: sourceType, Data: data, BaseURLOverride: baseURLOverride,
	})
	if err != nil {
		return importErrorResponse(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(job)
}

func (h *ImportHandler) GetJob(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleViewer)
	if !ok {
		return nil
	}
	jobID, err := parseIDParam(c, "jobId")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	job, err := h.service.GetImportJob(c.UserContext(), project.ID, jobID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load import job"})
	}
	if job == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "import job not found"})
	}
	return c.JSON(job)
}

type commitImportRequest struct {
	Selections []models.ImportCommitSelection `json:"selections"`
}

func (h *ImportHandler) Commit(c *fiber.Ctx) error {
	project, ok := authorizeProject(c, h.service, models.ProjectRoleEditor)
	if !ok {
		return nil
	}
	jobID, err := parseIDParam(c, "jobId")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	var req commitImportRequest
	if parseErr := c.BodyParser(&req); parseErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request payload"})
	}
	selections := make(map[string]models.ImportCommitSelection, len(req.Selections))
	for _, s := range req.Selections {
		selections[s.Key] = s
	}
	job, err := h.service.CommitImport(c.UserContext(), project, jobID, selections)
	if err != nil {
		return importErrorResponse(c, err)
	}
	return c.Status(fiber.StatusOK).JSON(job)
}

func importErrorResponse(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, application.ErrSpecTooLarge):
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrImportJobNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrImportJobCommitted):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, openapi.ErrTooLarge), errors.Is(err, openapi.ErrUnsupportedInput), errors.Is(err, openapi.ErrUnsupportedSpec),
		errors.Is(err, openapi.ErrNoPaths), errors.Is(err, openapi.ErrTooManyOperations), errors.Is(err, openapi.ErrRefBudgetExceeded),
		errors.Is(err, domain.ErrInvalidInput):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "import failed"})
	}
}
