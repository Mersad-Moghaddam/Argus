package ports

import (
	"context"
	"time"

	"argus/internal/models"
)

// UserStore persists user accounts.
type UserStore interface {
	CreateUser(ctx context.Context, user models.User) (int64, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id int64) (*models.User, error)
	UpdateUserPassword(ctx context.Context, id int64, passwordHash string) error
}

// AuthTokenStore persists opaque bearer session tokens.
type AuthTokenStore interface {
	CreateToken(ctx context.Context, token models.AuthToken) (int64, error)
	GetTokenByHash(ctx context.Context, tokenHash string) (*models.AuthToken, error)
	ListTokensByUser(ctx context.Context, userID int64) ([]models.AuthToken, error)
	TouchToken(ctx context.Context, id int64, usedAt time.Time) error
	DeleteToken(ctx context.Context, tokenHash string) error
	DeleteTokensByUserExcept(ctx context.Context, userID int64, tokenHash string) error
}

// ProjectStore persists projects and their membership/authorization data.
type ProjectStore interface {
	CreateProject(ctx context.Context, project models.Project, ownerUserID int64) (int64, error)
	UpdateProject(ctx context.Context, project models.Project) error
	SetProjectStatus(ctx context.Context, id int64, status string) error
	DeleteProject(ctx context.Context, id int64) error
	GetProjectByID(ctx context.Context, id int64) (*models.Project, error)
	ListProjects(ctx context.Context, userID int64, filter models.ProjectFilter) ([]models.Project, int, error)
	GetProjectMember(ctx context.Context, projectID, userID int64) (*models.ProjectMember, error)
	AddProjectMember(ctx context.Context, member models.ProjectMember) error
	ListProjectEnvironments(ctx context.Context, projectID int64) ([]models.ProjectEnvironment, error)
	CreateProjectEnvironment(ctx context.Context, environment models.ProjectEnvironment) (int64, error)
}

// TelemetryCredentialStore persists server-side OTLP ingestion credentials.
// Project and environment attribution is read from this store rather than
// trusted from inbound telemetry resource attributes.
type TelemetryCredentialStore interface {
	CreateTelemetryCredential(ctx context.Context, credential models.TelemetryCredential) (int64, error)
	ListTelemetryCredentials(ctx context.Context, projectID int64) ([]models.TelemetryCredential, error)
	GetTelemetryCredentialByID(ctx context.Context, id int64) (*models.TelemetryCredential, error)
	GetTelemetryCredentialByHash(ctx context.Context, tokenHash []byte) (*models.TelemetryCredential, error)
	RevokeTelemetryCredential(ctx context.Context, id int64, revokedAt time.Time) error
	TouchTelemetryCredential(ctx context.Context, id int64, usedAt time.Time) error
}

// TelemetryIngressStore retains bounded, non-sensitive receiver diagnostics.
// It must never be used as a high-volume time-series sample store.
type TelemetryIngressStore interface {
	RecordTelemetryIngress(ctx context.Context, record models.TelemetryIngressRecord) error
	ListTelemetryIngress(ctx context.Context, projectID int64, limit int) ([]models.TelemetryIngressRecord, error)
}

// RouteStore persists monitored API routes.
type RouteStore interface {
	CreateRoute(ctx context.Context, route models.APIRoute) (int64, error)
	BulkCreateRoutes(ctx context.Context, routes []models.APIRoute) (int, error)
	UpdateRoute(ctx context.Context, route models.APIRoute) error
	UpdateRouteImportedMetadata(ctx context.Context, route models.APIRoute) error
	SetRouteEnabled(ctx context.Context, id int64, enabled bool) error
	DeleteRoute(ctx context.Context, id int64) error
	BulkDeleteRoutes(ctx context.Context, projectID int64, ids []int64) (int64, error)
	GetRouteByID(ctx context.Context, id int64) (*models.APIRoute, error)
	GetRouteByMethodPath(ctx context.Context, projectID int64, method, path string) (*models.APIRoute, error)
	ListRoutes(ctx context.Context, filter models.RouteFilter) ([]models.APIRoute, int, error)
	ListAllRouteKeys(ctx context.Context, projectID int64) (map[string]int64, error)
	ListRouteSpecHashes(ctx context.Context, projectID int64) (map[int64]string, error)
	ListDueRoutes(ctx context.Context, now time.Time, limit int, afterID int64) ([]models.APIRoute, error)
	MarkRouteChecked(ctx context.Context, id int64, status string, statusCode, latencyMS int, failureReason string, consecutiveFailures, consecutiveSuccesses int, routeStatus string, checkedAt, nextCheckAt time.Time) error
	RecordRouteCheck(ctx context.Context, check models.RouteCheck) error
	ListRouteChecks(ctx context.Context, routeID int64, limit, offset int) ([]models.RouteCheck, error)
	// AggregateCheckTimeseries returns bucketed check statistics for a
	// project (or a single route within it). maxBuckets bounds the result so
	// a wide range can never produce an unbounded response.
	AggregateCheckTimeseries(ctx context.Context, projectID int64, routeID *int64, since time.Time, bucketSeconds, maxBuckets int) ([]models.MetricPoint, error)
	AggregateRouteMetrics(ctx context.Context, since time.Time) error
	AggregateProjectMetrics(ctx context.Context) error
	PruneRouteChecks(ctx context.Context, before time.Time, batchSize int) (int64, error)
}

// RouteIncidentStore persists route-level incidents.
type RouteIncidentStore interface {
	GetOpenRouteIncident(ctx context.Context, routeID int64) (*models.RouteIncident, error)
	CreateRouteIncident(ctx context.Context, routeID, projectID int64, reason string, startedAt time.Time) (int64, error)
	ResolveRouteIncident(ctx context.Context, incidentID int64, resolvedAt time.Time) error
	ListRouteIncidents(ctx context.Context, projectID int64, routeID *int64, state string, limit, offset int) ([]models.RouteIncident, error)
}

// ImportStore persists OpenAPI/Swagger import jobs.
type ImportStore interface {
	CreateImportJob(ctx context.Context, job models.ImportJob) (int64, error)
	GetImportJob(ctx context.Context, id int64) (*models.ImportJob, error)
	UpdateImportJob(ctx context.Context, job models.ImportJob) error
}
