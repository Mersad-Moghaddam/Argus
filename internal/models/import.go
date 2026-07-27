package models

import "time"

const (
	ImportSourceFile  = "file"
	ImportSourcePaste = "paste"

	ImportFormatOpenAPI3 = "openapi3"
	ImportFormatSwagger2 = "swagger2"

	ImportStatusValidated = "validated"
	ImportStatusCommitted = "committed"
	ImportStatusFailed    = "failed"

	ImportActionCreate = "create"
	ImportActionUpdate = "update"
	ImportActionSkip   = "skip"
	ImportActionRemove = "remove"

	ImportConflictNone      = "none"
	ImportConflictDuplicate = "duplicate_in_spec"
	ImportConflictChanged   = "changed"
	ImportConflictRemoved   = "removed_from_spec"
)

// ImportRouteItem is one candidate route discovered while parsing a spec,
// diffed against the project's existing routes.
type ImportRouteItem struct {
	Key               string   `json:"key"` // METHOD path
	Method            string   `json:"method"`
	Path              string   `json:"path"`
	BaseURL           string   `json:"baseUrl,omitempty"`
	OperationID       string   `json:"operationId,omitempty"`
	Summary           string   `json:"summary,omitempty"`
	Description       string   `json:"description,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	Deprecated        bool     `json:"deprecated"`
	Parameters        string   `json:"parameters,omitempty"`
	RequestBody       string   `json:"requestBody,omitempty"`
	Responses         string   `json:"responses,omitempty"`
	Security          string   `json:"security,omitempty"`
	SpecHash          string   `json:"specHash"`
	Action            string   `json:"action"`
	Conflict          string   `json:"conflict"`
	Selected          bool     `json:"selected"`
	ExistingRouteID   int64    `json:"existingRouteId,omitempty"`
	ValidationWarning string   `json:"validationWarning,omitempty"`
}

// ImportJob tracks a full validate -> preview -> commit lifecycle.
type ImportJob struct {
	ID              int64             `json:"id"`
	ProjectID       int64             `json:"projectId"`
	CreatedByUserID int64             `json:"createdByUserId"`
	SourceType      string            `json:"sourceType"`
	SpecFormat      string            `json:"specFormat"`
	Status          string            `json:"status"`
	Items           []ImportRouteItem `json:"items,omitempty"`
	TotalParsed     int               `json:"totalParsed"`
	CreatedRoutes   int               `json:"createdRoutes"`
	UpdatedRoutes   int               `json:"updatedRoutes"`
	SkippedRoutes   int               `json:"skippedRoutes"`
	RemovedRoutes   int               `json:"removedRoutes"`
	ErrorMessage    string            `json:"errorMessage,omitempty"`
	CreatedAt       time.Time         `json:"createdAt"`
	UpdatedAt       time.Time         `json:"updatedAt"`
}

// ImportCommitSelection is the per-item decision the user confirms before commit.
type ImportCommitSelection struct {
	Key      string `json:"key"`
	Selected bool   `json:"selected"`
	Action   string `json:"action"`
}

// RouteImportState is the minimal existing-route state needed to decide
// whether an OpenAPI re-import must update a route. The server/base URL is
// intentionally compared separately from SpecHash because it belongs to the
// top-level OpenAPI servers declaration, not to an individual operation.
type RouteImportState struct {
	SpecHash            string
	BaseURL             string
	ExpectedStatusRange string
	Source              string
}
