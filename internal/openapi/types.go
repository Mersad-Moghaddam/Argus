// Package openapi parses OpenAPI 3.x and Swagger 2.0 documents (JSON or
// YAML) into a flat list of monitorable routes. It is intentionally
// defensive: input is untrusted (user-uploaded specs), so parsing enforces
// size/complexity limits and never performs any network I/O (no remote
// $ref fetching).
package openapi

import "errors"

const (
	FormatOpenAPI3 = "openapi3"
	FormatSwagger2 = "swagger2"
)

// Limits protect against malicious or pathological documents.
const (
	MaxDocumentBytes  = 10 * 1024 * 1024 // 10MB raw spec size
	MaxOperations     = 5000             // hard cap on number of routes parsed
	MaxRefResolutions = 200000           // guards against ref-expansion blowups
	MaxRefDepth       = 60
)

var (
	ErrTooLarge          = errors.New("specification exceeds the maximum allowed size")
	ErrUnsupportedInput  = errors.New("unable to parse document as JSON or YAML")
	ErrUnsupportedSpec   = errors.New("document is not a recognizable OpenAPI 3.x or Swagger 2.0 specification")
	ErrNoPaths           = errors.New("specification does not define any paths")
	ErrTooManyOperations = errors.New("specification defines too many operations")
	ErrRefBudgetExceeded = errors.New("specification reference graph is too large to resolve safely")
)

// Parameter is a normalized request parameter definition.
type Parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"` // path|query|header|cookie|body|formData
	Required    bool   `json:"required"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	Example     string `json:"example,omitempty"`
	Default     string `json:"default,omitempty"`
}

// Route is one monitorable operation extracted from a spec.
type Route struct {
	Method      string      `json:"method"`
	Path        string      `json:"path"`
	OperationID string      `json:"operationId,omitempty"`
	Summary     string      `json:"summary,omitempty"`
	Description string      `json:"description,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Deprecated  bool        `json:"deprecated"`
	Parameters  []Parameter `json:"parameters,omitempty"`
	RequestBody any         `json:"requestBody,omitempty"`
	Responses   any         `json:"responses,omitempty"`
	Security    any         `json:"security,omitempty"`
	SpecHash    string      `json:"specHash"`
}

// Result is the outcome of parsing a specification document.
type Result struct {
	Format   string   `json:"format"`
	BaseURL  string   `json:"baseUrl,omitempty"`
	Title    string   `json:"title,omitempty"`
	Version  string   `json:"version,omitempty"`
	Routes   []Route  `json:"routes"`
	Warnings []string `json:"warnings,omitempty"`
}
