package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
	"argus/internal/openapi"
)

var ErrSpecTooLarge = errors.New("specification exceeds the maximum allowed upload size")

type ValidateImportInput struct {
	ProjectID       int64
	UserID          int64
	SourceType      string
	Data            []byte
	BaseURLOverride string
}

// ValidateImport parses an untrusted OpenAPI/Swagger document, diffs it
// against the project's existing routes, and persists a preview job. No
// routes are created or modified at this stage.
func (s *Service) ValidateImport(ctx context.Context, in ValidateImportInput) (models.ImportJob, error) {
	if len(in.Data) > openapi.MaxDocumentBytes {
		return models.ImportJob{}, ErrSpecTooLarge
	}
	if in.SourceType != models.ImportSourceFile && in.SourceType != models.ImportSourcePaste {
		return models.ImportJob{}, domain.ErrInvalidInput
	}

	parsed, err := openapi.Parse(in.Data)
	if err != nil {
		return models.ImportJob{}, err
	}

	baseURL := strings.TrimSpace(in.BaseURLOverride)
	if baseURL == "" {
		baseURL = parsed.BaseURL
	}
	if baseURL != "" {
		var baseErr error
		baseURL, _, baseErr = domain.NormalizeBaseURL(baseURL)
		if baseErr != nil {
			return models.ImportJob{}, baseErr
		}
	}

	existingIDs, err := s.routes.ListAllRouteKeys(ctx, in.ProjectID)
	if err != nil {
		return models.ImportJob{}, err
	}
	existingHashes, err := s.routes.ListRouteSpecHashes(ctx, in.ProjectID)
	if err != nil {
		return models.ImportJob{}, err
	}

	seenInSpec := map[string]bool{}
	items := make([]models.ImportRouteItem, 0, len(parsed.Routes))
	for _, route := range parsed.Routes {
		normalized, normalizeErr := domain.NormalizeEndpoint(route.Method, baseURL, route.Path)
		item := models.ImportRouteItem{
			Method: route.Method, Path: route.Path, BaseURL: baseURL,
			OperationID: route.OperationID, Summary: route.Summary, Description: route.Description,
			Tags: route.Tags, Deprecated: route.Deprecated, SpecHash: route.SpecHash,
		}
		item.Parameters = marshalOrEmpty(route.Parameters)
		item.RequestBody = marshalOrEmpty(route.RequestBody)
		item.Responses = marshalOrEmpty(route.Responses)
		item.Security = marshalOrEmpty(route.Security)

		if normalizeErr != nil {
			item.Action = models.ImportActionSkip
			item.Conflict = models.ImportConflictNone
			item.ValidationWarning = "invalid method or path, skipped"
			items = append(items, item)
			continue
		}
		item.Method, item.Path, item.BaseURL = normalized.Method, normalized.RouteTemplate, normalized.BaseURL
		item.Key = item.Method + " " + item.Path

		if seenInSpec[item.Key] {
			item.Action = models.ImportActionSkip
			item.Conflict = models.ImportConflictDuplicate
			item.Selected = false
			items = append(items, item)
			continue
		}
		seenInSpec[item.Key] = true

		if existingID, ok := existingIDs[item.Key]; ok {
			item.ExistingRouteID = existingID
			if existingHashes[existingID] == route.SpecHash {
				item.Action = models.ImportActionSkip
				item.Conflict = models.ImportConflictNone
				item.Selected = false
			} else {
				item.Action = models.ImportActionUpdate
				item.Conflict = models.ImportConflictChanged
				item.Selected = true
			}
		} else {
			item.Action = models.ImportActionCreate
			item.Conflict = models.ImportConflictNone
			item.Selected = true
		}
		items = append(items, item)
	}

	// Routes that exist in the project but were not present in this spec.
	for key, id := range existingIDs {
		if seenInSpec[key] {
			continue
		}
		parts := strings.SplitN(key, " ", 2)
		method, path := key, ""
		if len(parts) == 2 {
			method, path = parts[0], parts[1]
		}
		items = append(items, models.ImportRouteItem{
			Key: key, Method: method, Path: path, ExistingRouteID: id,
			Action: models.ImportActionRemove, Conflict: models.ImportConflictRemoved, Selected: false,
		})
	}

	job := models.ImportJob{
		ProjectID: in.ProjectID, CreatedByUserID: in.UserID, SourceType: in.SourceType,
		SpecFormat: parsed.Format, Status: models.ImportStatusValidated, Items: items, TotalParsed: len(parsed.Routes),
	}
	id, err := s.imports.CreateImportJob(ctx, job)
	if err != nil {
		return models.ImportJob{}, err
	}
	job.ID = id
	return job, nil
}

func marshalOrEmpty(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// CommitImport applies the user-confirmed selection from a validated import
// job: creates new routes, updates spec-derived metadata on changed routes
// (never touching user-owned monitoring configuration), and disables routes
// that disappeared from the spec if the user explicitly opted into that.
// Every row is processed independently so a single bad row cannot abort the
// whole batch; failures are reported in the final result.
func (s *Service) CommitImport(ctx context.Context, project models.Project, jobID int64, selections map[string]models.ImportCommitSelection) (models.ImportJob, error) {
	job, err := s.imports.GetImportJob(ctx, jobID)
	if err != nil {
		return models.ImportJob{}, err
	}
	if job == nil || job.ProjectID != project.ID {
		return models.ImportJob{}, domain.ErrImportJobNotFound
	}
	if job.Status == models.ImportStatusCommitted {
		return models.ImportJob{}, domain.ErrImportJobCommitted
	}

	toCreate := []models.APIRoute{}
	finalItems := make([]models.ImportRouteItem, 0, len(job.Items))
	var created, updated, skipped, removed int

	for _, item := range job.Items {
		if sel, ok := selections[item.Key]; ok {
			item.Selected = sel.Selected
		}
		if !item.Selected {
			item.Action = models.ImportActionSkip
			skipped++
			finalItems = append(finalItems, item)
			continue
		}

		switch item.Action {
		case models.ImportActionCreate:
			normalized, normalizeErr := domain.NormalizeEndpoint(item.Method, item.BaseURL, item.Path)
			if normalizeErr != nil {
				item.ValidationWarning = "route could not be canonically normalized, skipped"
				skipped++
				finalItems = append(finalItems, item)
				continue
			}
			route := models.APIRoute{
				ProjectID: project.ID, Method: normalized.Method, Path: normalized.RouteTemplate, BaseURL: normalized.BaseURL,
				CanonicalIdentity: normalized.CanonicalIdentity, CanonicalHash: domain.CanonicalHash(normalized.CanonicalIdentity), CanonicalVersion: 1,
				OperationID: item.OperationID, Summary: item.Summary, Description: item.Description,
				Tags: item.Tags, Deprecated: item.Deprecated, SpecHash: item.SpecHash,
				Parameters: item.Parameters, RequestBody: item.RequestBody, Responses: item.Responses, Security: item.Security,
				// OpenAPI is a catalog source. Importing it must not schedule a
				// request, regardless of operation method or prior defaults.
				Source: "import", Enabled: false,
				MonitorIntervalSecs: project.DefaultIntervalSeconds, TimeoutMS: project.DefaultTimeoutMS, Retries: project.DefaultRetries,
				ExpectedStatusRange: "200-399", FailureThreshold: project.FailureThreshold, RecoverySuccesses: project.RecoverySuccessThreshold,
				Status: domain.RouteStatusUnknown, NextCheckAt: time.Now().UTC(),
			}
			toCreate = append(toCreate, route)
			created++
		case models.ImportActionUpdate:
			existing, getErr := s.routes.GetRouteByID(ctx, item.ExistingRouteID)
			if getErr != nil || existing == nil {
				item.ValidationWarning = "route no longer exists, skipped"
				skipped++
				finalItems = append(finalItems, item)
				continue
			}
			existing.Name = item.OperationID
			existing.Summary = item.Summary
			existing.Description = item.Description
			existing.Tags = item.Tags
			existing.Deprecated = item.Deprecated
			existing.Parameters = item.Parameters
			existing.RequestBody = item.RequestBody
			existing.Responses = item.Responses
			existing.Security = item.Security
			existing.SpecHash = item.SpecHash
			if item.BaseURL != "" {
				existing.BaseURL = item.BaseURL
			}
			normalized, normalizeErr := domain.NormalizeEndpoint(existing.Method, existing.BaseURL, existing.Path)
			if normalizeErr != nil {
				item.ValidationWarning = "route could not be canonically normalized, skipped"
				skipped++
				finalItems = append(finalItems, item)
				continue
			}
			existing.CanonicalIdentity = normalized.CanonicalIdentity
			existing.CanonicalHash = domain.CanonicalHash(normalized.CanonicalIdentity)
			existing.CanonicalVersion = 1
			if updErr := s.routes.UpdateRouteImportedMetadata(ctx, *existing); updErr != nil {
				item.ValidationWarning = fmt.Sprintf("update failed: %v", updErr)
				skipped++
				finalItems = append(finalItems, item)
				continue
			}
			updated++
		case models.ImportActionRemove:
			if disErr := s.routes.SetRouteEnabled(ctx, item.ExistingRouteID, false); disErr != nil {
				item.ValidationWarning = fmt.Sprintf("disable failed: %v", disErr)
				skipped++
				finalItems = append(finalItems, item)
				continue
			}
			removed++
		default:
			skipped++
		}
		finalItems = append(finalItems, item)
	}

	if len(toCreate) > 0 {
		if _, err = s.routes.BulkCreateRoutes(ctx, toCreate); err != nil {
			return models.ImportJob{}, err
		}
	}

	job.Items = finalItems
	job.Status = models.ImportStatusCommitted
	job.CreatedRoutes = created
	job.UpdatedRoutes = updated
	job.SkippedRoutes = skipped
	job.RemovedRoutes = removed
	if err = s.imports.UpdateImportJob(ctx, *job); err != nil {
		return models.ImportJob{}, err
	}
	s.logger.Add("info", "api", "import_committed", "OpenAPI import committed", nil, map[string]string{
		"projectId": fmt.Sprintf("%d", project.ID), "created": fmt.Sprintf("%d", created),
		"updated": fmt.Sprintf("%d", updated), "disabled": fmt.Sprintf("%d", removed), "skipped": fmt.Sprintf("%d", skipped),
	})
	return *job, nil
}

func (s *Service) GetImportJob(ctx context.Context, projectID, jobID int64) (*models.ImportJob, error) {
	job, err := s.imports.GetImportJob(ctx, jobID)
	if err != nil || job == nil || job.ProjectID != projectID {
		return nil, err
	}
	return job, nil
}
