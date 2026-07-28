// Package testsupport provides hand-rolled, in-memory implementations of the
// project-monitoring repository ports.
//
// It exists so the application-service tests and the HTTP handler tests can
// share one set of test doubles instead of maintaining two copies. It is
// imported only from _test.go files and is therefore never linked into the
// cmd/api binary; it deliberately depends on nothing above the domain and
// models layers so it cannot create an import cycle with the packages under
// test.
//
// The fakes mirror the MySQL adapters' observable semantics — uniqueness on
// (project, method, path), filtering/sorting/pagination in ListRoutes,
// project-scoped bulk deletes, and "not found" reported as a nil result
// rather than an error — so tests written against them exercise the same
// contract production code relies on.
package testsupport

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"argus/internal/domain"
	"argus/internal/domain/ports"
	"argus/internal/models"
)

// Stores bundles one instance of every fake so a test can seed and assert on
// them while handing the same values to application.NewService.
type Stores struct {
	Users                *UserStore
	Tokens               *AuthTokenStore
	PasswordRecovery     *PasswordRecoveryStore
	RecoveryDelivery     *RecoveryDelivery
	Projects             *ProjectStore
	Routes               *RouteStore
	Incidents            *RouteIncidentStore
	Imports              *ImportStore
	TelemetryCredentials *TelemetryCredentialStore
	TelemetryIngress     *TelemetryIngressStore
	TelemetryMappings    *TelemetryMappingStore
	SLOs                 *SLOStore
	Outbox               *OutboxStore
	Legacy               LegacyStore
}

// NewStores returns a fresh, empty set of fakes.
func NewStores() *Stores {
	return &Stores{
		Users:                NewUserStore(),
		Tokens:               NewAuthTokenStore(),
		PasswordRecovery:     NewPasswordRecoveryStore(),
		RecoveryDelivery:     &RecoveryDelivery{},
		Projects:             NewProjectStore(),
		Routes:               NewRouteStore(),
		Incidents:            NewRouteIncidentStore(),
		Imports:              NewImportStore(),
		TelemetryCredentials: NewTelemetryCredentialStore(),
		TelemetryIngress:     NewTelemetryIngressStore(),
		TelemetryMappings:    NewTelemetryMappingStore(),
		SLOs:                 NewSLOStore(),
		Outbox:               &OutboxStore{},
	}
}

// ---------------------------------------------------------------- telemetry credentials

type TelemetryCredentialStore struct {
	mu     sync.Mutex
	nextID int64
	byID   map[int64]models.TelemetryCredential
}

func NewTelemetryCredentialStore() *TelemetryCredentialStore {
	return &TelemetryCredentialStore{byID: map[int64]models.TelemetryCredential{}}
}

func (f *TelemetryCredentialStore) CreateTelemetryCredential(_ context.Context, credential models.TelemetryCredential) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	credential.ID = f.nextID
	credential.CreatedAt = time.Now().UTC()
	credential.UpdatedAt = credential.CreatedAt
	f.byID[credential.ID] = credential
	return credential.ID, nil
}

func (f *TelemetryCredentialStore) ListTelemetryCredentials(_ context.Context, projectID int64) ([]models.TelemetryCredential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]models.TelemetryCredential, 0)
	for _, credential := range f.byID {
		if credential.ProjectID == projectID {
			items = append(items, copyTelemetryCredential(credential))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	return items, nil
}

func (f *TelemetryCredentialStore) GetTelemetryCredentialByID(_ context.Context, id int64) (*models.TelemetryCredential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	credential, ok := f.byID[id]
	if !ok {
		return nil, nil
	}
	copied := copyTelemetryCredential(credential)
	return &copied, nil
}

func (f *TelemetryCredentialStore) GetTelemetryCredentialByHash(_ context.Context, tokenHash []byte) (*models.TelemetryCredential, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, credential := range f.byID {
		if string(credential.TokenHash) == string(tokenHash) {
			copied := copyTelemetryCredential(credential)
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *TelemetryCredentialStore) RevokeTelemetryCredential(_ context.Context, id int64, revokedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	credential, ok := f.byID[id]
	if !ok || credential.RevokedAt != nil {
		return nil
	}
	credential.RevokedAt = &revokedAt
	credential.UpdatedAt = revokedAt
	f.byID[id] = credential
	return nil
}

func (f *TelemetryCredentialStore) TouchTelemetryCredential(_ context.Context, id int64, usedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	credential, ok := f.byID[id]
	if !ok || credential.RevokedAt != nil {
		return nil
	}
	credential.LastUsedAt = &usedAt
	credential.UpdatedAt = usedAt
	f.byID[id] = credential
	return nil
}

func copyTelemetryCredential(in models.TelemetryCredential) models.TelemetryCredential {
	in.TokenHash = append([]byte(nil), in.TokenHash...)
	return in
}

type TelemetryIngressStore struct {
	mu      sync.Mutex
	nextID  int64
	records []models.TelemetryIngressRecord
}

type TelemetryMappingStore struct {
	mu     sync.Mutex
	nextID int64
	items  []models.TelemetryRouteMapping
}

func NewTelemetryMappingStore() *TelemetryMappingStore { return &TelemetryMappingStore{} }
func (f *TelemetryMappingStore) CreateTelemetryRouteMapping(_ context.Context, m models.TelemetryRouteMapping) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	m.ID = f.nextID
	m.CreatedAt = time.Now().UTC()
	m.UpdatedAt = m.CreatedAt
	f.items = append(f.items, m)
	return m.ID, nil
}
func (f *TelemetryMappingStore) ListTelemetryRouteMappings(_ context.Context, pid int64) ([]models.TelemetryRouteMapping, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []models.TelemetryRouteMapping{}
	for i := len(f.items) - 1; i >= 0; i-- {
		if f.items[i].ProjectID == pid {
			out = append(out, f.items[i])
		}
	}
	return out, nil
}
func (f *TelemetryMappingStore) DeleteTelemetryRouteMapping(_ context.Context, pid, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.items[:0]
	for _, m := range f.items {
		if m.ID != id || m.ProjectID != pid {
			out = append(out, m)
		}
	}
	f.items = out
	return nil
}

type SLOStore struct {
	mu          sync.Mutex
	nextID      int64
	nextEvalID  int64
	definitions map[int64]models.SLODefinition
	evaluations []models.SLOEvaluation
}

func NewSLOStore() *SLOStore { return &SLOStore{definitions: map[int64]models.SLODefinition{}} }
func (f *SLOStore) CreateSLODefinition(_ context.Context, definition models.SLODefinition) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.definitions {
		if existing.ProjectID == definition.ProjectID && existing.Name == definition.Name {
			return 0, domain.ErrInvalidInput
		}
	}
	f.nextID++
	definition.ID, definition.Version = f.nextID, 1
	definition.CreatedAt = time.Now().UTC()
	definition.UpdatedAt = definition.CreatedAt
	f.definitions[definition.ID] = definition
	return definition.ID, nil
}
func (f *SLOStore) GetSLODefinition(_ context.Context, projectID, id int64) (*models.SLODefinition, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.definitions[id]
	if !ok || item.ProjectID != projectID {
		return nil, nil
	}
	return &item, nil
}
func (f *SLOStore) ListSLODefinitions(_ context.Context, projectID int64) ([]models.SLODefinition, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := []models.SLODefinition{}
	for _, item := range f.definitions {
		if item.ProjectID == projectID {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID > items[j].ID })
	return items, nil
}
func (f *SLOStore) ListSLODefinitionsForEvaluation(_ context.Context, limit, afterID int64) ([]models.SLODefinition, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	items := []models.SLODefinition{}
	for _, item := range f.definitions {
		if item.ID > afterID {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	if int64(len(items)) > limit {
		items = items[:int(limit)]
	}
	return items, nil
}
func (f *SLOStore) RecordSLOEvaluation(_ context.Context, evaluation models.SLOEvaluation) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextEvalID++
	evaluation.ID = f.nextEvalID
	f.evaluations = append(f.evaluations, evaluation)
	return evaluation.ID, nil
}
func (f *SLOStore) ListSLOEvaluations(_ context.Context, projectID, sloID int64, limit int) ([]models.SLOEvaluation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	items := []models.SLOEvaluation{}
	for i := len(f.evaluations) - 1; i >= 0 && len(items) < limit; i-- {
		if item := f.evaluations[i]; item.ProjectID == projectID && item.SLOID == sloID {
			items = append(items, item)
		}
	}
	return items, nil
}

func NewTelemetryIngressStore() *TelemetryIngressStore { return &TelemetryIngressStore{} }

func (f *TelemetryIngressStore) RecordTelemetryIngress(_ context.Context, record models.TelemetryIngressRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	record.ID = f.nextID
	f.records = append(f.records, record)
	return nil
}

func (f *TelemetryIngressStore) ListTelemetryIngress(_ context.Context, projectID int64, limit int) ([]models.TelemetryIngressRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	items := make([]models.TelemetryIngressRecord, 0, limit)
	for i := len(f.records) - 1; i >= 0 && len(items) < limit; i-- {
		if f.records[i].ProjectID == projectID {
			items = append(items, f.records[i])
		}
	}
	return items, nil
}

// ---------------------------------------------------------------- users

type UserStore struct {
	mu     sync.Mutex
	nextID int64
	byID   map[int64]models.User
	Err    error // when set, every call fails; used to test error propagation
}

func NewUserStore() *UserStore { return &UserStore{byID: map[int64]models.User{}} }

func (f *UserStore) CreateUser(_ context.Context, user models.User) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return 0, f.Err
	}
	f.nextID++
	user.ID = f.nextID
	user.CreatedAt = time.Now().UTC()
	f.byID[user.ID] = user
	return user.ID, nil
}

func (f *UserStore) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return nil, f.Err
	}
	for _, u := range f.byID {
		if strings.EqualFold(u.Email, email) {
			copied := u
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *UserStore) GetUserByID(_ context.Context, id int64) (*models.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if u, ok := f.byID[id]; ok {
		copied := u
		return &copied, nil
	}
	return nil, nil
}

func (f *UserStore) UpdateUserPassword(_ context.Context, id int64, passwordHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	user, ok := f.byID[id]
	if !ok {
		return nil
	}
	user.PasswordHash = passwordHash
	user.UpdatedAt = time.Now().UTC()
	f.byID[id] = user
	return nil
}

// ---------------------------------------------------------------- tokens

type AuthTokenStore struct {
	mu      sync.Mutex
	nextID  int64
	byHash  map[string]models.AuthToken
	Touched int
	Deleted []string
}

// PasswordRecoveryStore is an in-memory atomic consume implementation for
// recovery workflow tests. It mirrors the MySQL used_at predicate.
type PasswordRecoveryStore struct {
	mu     sync.Mutex
	nextID int64
	byHash map[string]models.PasswordRecoveryToken
}

// RecoveryDelivery records reset deliveries without retaining them in a
// production store. Tests use the raw token to exercise one-time completion.
type RecoveryDelivery struct {
	mu        sync.Mutex
	Email     string
	Token     string
	ExpiresAt time.Time
	Err       error
}

func (f *RecoveryDelivery) DeliverPasswordRecovery(_ context.Context, email, token string, expiresAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Err != nil {
		return f.Err
	}
	f.Email, f.Token, f.ExpiresAt = email, token, expiresAt
	return nil
}

func NewPasswordRecoveryStore() *PasswordRecoveryStore {
	return &PasswordRecoveryStore{byHash: map[string]models.PasswordRecoveryToken{}}
}

func (f *PasswordRecoveryStore) CreatePasswordRecoveryToken(_ context.Context, token models.PasswordRecoveryToken) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	token.ID = f.nextID
	token.CreatedAt = time.Now().UTC()
	f.byHash[token.TokenHash] = token
	return token.ID, nil
}

func (f *PasswordRecoveryStore) ConsumePasswordRecoveryToken(_ context.Context, hash string, usedAt time.Time) (*models.PasswordRecoveryToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	token, ok := f.byHash[hash]
	if !ok || token.UsedAt != nil || !token.ExpiresAt.After(usedAt) {
		return nil, nil
	}
	token.UsedAt = &usedAt
	f.byHash[hash] = token
	return &token, nil
}

func (f *PasswordRecoveryStore) DeletePasswordRecoveryTokensByUser(_ context.Context, userID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for hash, token := range f.byHash {
		if token.UserID == userID {
			delete(f.byHash, hash)
		}
	}
	return nil
}

func NewAuthTokenStore() *AuthTokenStore {
	return &AuthTokenStore{byHash: map[string]models.AuthToken{}}
}

func (f *AuthTokenStore) CreateToken(_ context.Context, token models.AuthToken) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	token.ID = f.nextID
	f.byHash[token.TokenHash] = token
	return token.ID, nil
}

func (f *AuthTokenStore) GetTokenByHash(_ context.Context, hash string) (*models.AuthToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if t, ok := f.byHash[hash]; ok {
		copied := t
		return &copied, nil
	}
	return nil, nil
}

func (f *AuthTokenStore) ListTokensByUser(_ context.Context, userID int64) ([]models.AuthToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []models.AuthToken{}
	for _, token := range f.byHash {
		if token.UserID == userID {
			out = append(out, token)
		}
	}
	return out, nil
}

func (f *AuthTokenStore) TouchToken(_ context.Context, _ int64, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Touched++
	return nil
}

func (f *AuthTokenStore) DeleteToken(_ context.Context, hash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Deleted = append(f.Deleted, hash)
	delete(f.byHash, hash)
	return nil
}

func (f *AuthTokenStore) DeleteTokensByUserExcept(_ context.Context, userID int64, keepHash string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for hash, token := range f.byHash {
		if token.UserID == userID && hash != keepHash {
			f.Deleted = append(f.Deleted, hash)
			delete(f.byHash, hash)
		}
	}
	return nil
}

// ExpireAll backdates every issued token so expiry handling can be tested
// without waiting out the real 30-day TTL. Tokens are keyed by a hash derived
// by unexported service code, so tests cannot target one individually.
func (f *AuthTokenStore) ExpireAll() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for hash, token := range f.byHash {
		token.ExpiresAt = time.Now().UTC().Add(-time.Hour)
		f.byHash[hash] = token
	}
}

// Count reports how many tokens are currently issued.
func (f *AuthTokenStore) Count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.byHash)
}

// ---------------------------------------------------------------- projects

type ProjectStore struct {
	mu           sync.Mutex
	nextID       int64
	byID         map[int64]models.Project
	members      map[string]models.ProjectMember // "projectID:userID"
	environments map[int64][]models.ProjectEnvironment
}

func NewProjectStore() *ProjectStore {
	return &ProjectStore{byID: map[int64]models.Project{}, members: map[string]models.ProjectMember{}, environments: map[int64][]models.ProjectEnvironment{}}
}

func memberKey(projectID, userID int64) string {
	return strconv.FormatInt(projectID, 10) + ":" + strconv.FormatInt(userID, 10)
}

func (f *ProjectStore) CreateProject(_ context.Context, project models.Project, ownerUserID int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	project.ID = f.nextID
	project.OwnerUserID = ownerUserID
	project.CreatedAt = time.Now().UTC()
	f.byID[project.ID] = project
	f.members[memberKey(project.ID, ownerUserID)] = models.ProjectMember{ProjectID: project.ID, UserID: ownerUserID, Role: models.ProjectRoleOwner}
	f.environments[project.ID] = []models.ProjectEnvironment{{ID: 1, ProjectID: project.ID, Name: "production", IsDefault: true}}
	return project.ID, nil
}

func (f *ProjectStore) ListProjectEnvironments(_ context.Context, projectID int64) ([]models.ProjectEnvironment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]models.ProjectEnvironment(nil), f.environments[projectID]...), nil
}

func (f *ProjectStore) CreateProjectEnvironment(_ context.Context, env models.ProjectEnvironment) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := f.environments[env.ProjectID]
	env.ID = int64(len(items) + 1)
	f.environments[env.ProjectID] = append(items, env)
	return env.ID, nil
}

func (f *ProjectStore) UpdateProject(_ context.Context, project models.Project) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[project.ID]; !ok {
		return nil
	}
	f.byID[project.ID] = project
	return nil
}

func (f *ProjectStore) SetProjectStatus(_ context.Context, id int64, status string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.byID[id]; ok {
		p.Status = status
		f.byID[id] = p
	}
	return nil
}

func (f *ProjectStore) DeleteProject(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *ProjectStore) GetProjectByID(_ context.Context, id int64) (*models.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.byID[id]; ok {
		copied := p
		return &copied, nil
	}
	return nil, nil
}

func (f *ProjectStore) ListProjects(_ context.Context, userID int64, filter models.ProjectFilter) ([]models.Project, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []models.Project{}
	for _, p := range f.byID {
		if _, ok := f.members[memberKey(p.ID, userID)]; !ok {
			continue
		}
		if filter.Status != "" && p.Status != filter.Status {
			continue
		}
		if filter.Search != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(filter.Search)) {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	total := len(out)
	return pageSlice(out, filter.Limit, filter.Offset), total, nil
}

func (f *ProjectStore) GetProjectMember(_ context.Context, projectID, userID int64) (*models.ProjectMember, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if m, ok := f.members[memberKey(projectID, userID)]; ok {
		copied := m
		return &copied, nil
	}
	return nil, nil
}

func (f *ProjectStore) AddProjectMember(_ context.Context, member models.ProjectMember) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.members[memberKey(member.ProjectID, member.UserID)] = member
	return nil
}

// ---------------------------------------------------------------- routes

type RouteStore struct {
	mu     sync.Mutex
	nextID int64
	byID   map[int64]models.APIRoute
	checks map[int64][]models.RouteCheck

	// UpdateMetadataFailFor injects a storage failure for a specific route so
	// partial-failure reporting can be tested.
	UpdateMetadataFailFor map[int64]error
	// BulkCreateErr makes BulkCreateRoutes fail wholesale.
	BulkCreateErr error
	// ListErr makes ListRoutes fail, to exercise handler 500 paths.
	ListErr error
}

func NewRouteStore() *RouteStore {
	return &RouteStore{
		byID: map[int64]models.APIRoute{}, checks: map[int64][]models.RouteCheck{},
		UpdateMetadataFailFor: map[int64]error{},
	}
}

func routeKey(r models.APIRoute) string { return r.Method + " " + r.Path }

func (f *RouteStore) insertLocked(route models.APIRoute) int64 {
	f.nextID++
	route.ID = f.nextID
	route.CreatedAt = time.Now().UTC()
	route.UpdatedAt = route.CreatedAt
	f.byID[route.ID] = route
	return route.ID
}

func (f *RouteStore) CreateRoute(_ context.Context, route models.APIRoute) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, existing := range f.byID {
		if existing.ProjectID == route.ProjectID && routeKey(existing) == routeKey(route) {
			return 0, domain.ErrDuplicateRoute
		}
	}
	return f.insertLocked(route), nil
}

func (f *RouteStore) BulkCreateRoutes(_ context.Context, routes []models.APIRoute) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.BulkCreateErr != nil {
		return 0, f.BulkCreateErr
	}
	inserted := 0
	for _, route := range routes {
		duplicate := false
		for _, existing := range f.byID {
			if existing.ProjectID == route.ProjectID && routeKey(existing) == routeKey(route) {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue // mirrors the adapter's INSERT IGNORE semantics
		}
		f.insertLocked(route)
		inserted++
	}
	return inserted, nil
}

func (f *RouteStore) UpdateRoute(_ context.Context, route models.APIRoute) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[route.ID]; !ok {
		return domain.ErrRouteNotFound
	}
	route.UpdatedAt = time.Now().UTC()
	f.byID[route.ID] = route
	return nil
}

// UpdateRouteImportedMetadata mirrors the adapter: it writes ONLY spec-derived
// columns and never the user-owned monitoring configuration.
func (f *RouteStore) UpdateRouteImportedMetadata(_ context.Context, route models.APIRoute) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err, ok := f.UpdateMetadataFailFor[route.ID]; ok {
		return err
	}
	existing, ok := f.byID[route.ID]
	if !ok {
		return domain.ErrRouteNotFound
	}
	existing.Name = route.Name
	existing.Summary = route.Summary
	existing.Description = route.Description
	existing.Tags = route.Tags
	existing.Deprecated = route.Deprecated
	existing.Parameters = route.Parameters
	existing.RequestBody = route.RequestBody
	existing.Responses = route.Responses
	existing.Security = route.Security
	existing.SpecHash = route.SpecHash
	existing.BaseURL = route.BaseURL
	existing.UpdatedAt = time.Now().UTC()
	f.byID[route.ID] = existing
	return nil
}

func (f *RouteStore) SetRouteEnabled(_ context.Context, id int64, enabled bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	route, ok := f.byID[id]
	if !ok {
		return domain.ErrRouteNotFound
	}
	route.Enabled = enabled
	lastStatus := "up"
	if route.LastFailureReason != "" || route.LastStatusCode < 200 || route.LastStatusCode >= 400 {
		lastStatus = "down"
	}
	route.Status = domain.ComputeRouteStatus(domain.RouteHealthInput{
		Enabled: enabled, Checked: route.LastCheckedAt != nil, LastStatus: lastStatus,
		ConsecutiveFailures: route.ConsecutiveFailures, ConsecutiveSuccesses: route.ConsecutiveSuccesses,
		FailureThreshold: route.FailureThreshold,
	})
	f.byID[id] = route
	return nil
}

func (f *RouteStore) DeleteRoute(_ context.Context, id int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.byID, id)
	return nil
}

func (f *RouteStore) BulkDeleteRoutes(_ context.Context, projectID int64, ids []int64) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var deleted int64
	for _, id := range ids {
		// Scoping by project ID is the security-critical part of bulk delete.
		if route, ok := f.byID[id]; ok && route.ProjectID == projectID {
			delete(f.byID, id)
			deleted++
		}
	}
	return deleted, nil
}

func (f *RouteStore) GetRouteByID(_ context.Context, id int64) (*models.APIRoute, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.byID[id]; ok {
		copied := r
		return &copied, nil
	}
	return nil, nil
}

func (f *RouteStore) GetRouteByMethodPath(_ context.Context, projectID int64, method, path string) (*models.APIRoute, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.byID {
		if r.ProjectID == projectID && r.Method == method && r.Path == path {
			copied := r
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *RouteStore) ListRoutes(_ context.Context, filter models.RouteFilter) ([]models.APIRoute, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ListErr != nil {
		return nil, 0, f.ListErr
	}
	out := []models.APIRoute{}
	for _, r := range f.byID {
		if r.ProjectID != filter.ProjectID {
			continue
		}
		if filter.Search != "" {
			needle := strings.ToLower(filter.Search)
			if !strings.Contains(strings.ToLower(r.Path), needle) &&
				!strings.Contains(strings.ToLower(r.Summary), needle) &&
				!strings.Contains(strings.ToLower(r.Name), needle) {
				continue
			}
		}
		if filter.Method != "" && r.Method != filter.Method {
			continue
		}
		if filter.Status != "" && r.Status != filter.Status {
			continue
		}
		if filter.Tag != "" && !ContainsTag(r.Tags, filter.Tag) {
			continue
		}
		if filter.Enabled != nil && r.Enabled != *filter.Enabled {
			continue
		}
		if filter.Deprecated != nil && r.Deprecated != *filter.Deprecated {
			continue
		}
		out = append(out, r)
	}
	sortRoutes(out, filter.SortBy, filter.SortDir)
	total := len(out)
	return pageSlice(out, filter.Limit, filter.Offset), total, nil
}

// ContainsTag reports whether tags holds tag, case-insensitively.
func ContainsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func sortRoutes(routes []models.APIRoute, sortBy, sortDir string) {
	less := func(i, j int) bool { return routes[i].Path < routes[j].Path }
	switch sortBy {
	case "method":
		less = func(i, j int) bool { return routes[i].Method < routes[j].Method }
	case "status":
		less = func(i, j int) bool { return routes[i].Status < routes[j].Status }
	case "latency":
		less = func(i, j int) bool { return routes[i].AvgLatency24hMS < routes[j].AvgLatency24hMS }
	case "uptime":
		less = func(i, j int) bool { return routes[i].Uptime24hPct < routes[j].Uptime24hPct }
	}
	sort.SliceStable(routes, less)
	if strings.EqualFold(sortDir, "desc") {
		for i, j := 0, len(routes)-1; i < j; i, j = i+1, j-1 {
			routes[i], routes[j] = routes[j], routes[i]
		}
	}
}

func pageSlice[T any](items []T, limit, offset int) []T {
	if offset > 0 {
		if offset >= len(items) {
			return []T{}
		}
		items = items[offset:]
	}
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items
}

func (f *RouteStore) ListAllRouteKeys(_ context.Context, projectID int64) (map[string]int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]int64{}
	for _, r := range f.byID {
		if r.ProjectID == projectID {
			out[routeKey(r)] = r.ID
		}
	}
	return out, nil
}

func (f *RouteStore) ListRouteSpecHashes(_ context.Context, projectID int64) (map[int64]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[int64]string{}
	for _, r := range f.byID {
		if r.ProjectID == projectID {
			out[r.ID] = r.SpecHash
		}
	}
	return out, nil
}

func (f *RouteStore) ListDueRoutes(_ context.Context, now time.Time, limit int, afterID int64) ([]models.APIRoute, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []models.APIRoute{}
	for _, r := range f.byID {
		if r.Enabled && r.ID > afterID && !r.NextCheckAt.After(now) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

func (f *RouteStore) MarkRouteChecked(_ context.Context, id int64, _ string, statusCode, latencyMS int, failureReason string, consecutiveFailures, consecutiveSuccesses int, routeStatus string, checkedAt, nextCheckAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	route, ok := f.byID[id]
	if !ok {
		return domain.ErrRouteNotFound
	}
	at := checkedAt
	route.LastCheckedAt = &at
	route.LastStatusCode = statusCode
	route.LastLatencyMS = latencyMS
	route.LastFailureReason = failureReason
	route.ConsecutiveFailures = consecutiveFailures
	route.ConsecutiveSuccesses = consecutiveSuccesses
	route.Status = routeStatus
	route.NextCheckAt = nextCheckAt
	f.byID[id] = route
	return nil
}

func (f *RouteStore) RecordRouteCheck(_ context.Context, check models.RouteCheck) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	check.ID = int64(len(f.checks[check.RouteID]) + 1)
	f.checks[check.RouteID] = append(f.checks[check.RouteID], check)
	return nil
}

func (f *RouteStore) ListRouteChecks(_ context.Context, routeID int64, limit, offset int) ([]models.RouteCheck, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	all := f.checks[routeID]
	out := make([]models.RouteCheck, len(all))
	copy(out, all)
	sort.SliceStable(out, func(i, j int) bool { return out[i].CheckedAt.After(out[j].CheckedAt) })
	return pageSlice(out, limit, offset), nil
}

// AggregateCheckTimeseries buckets the recorded checks the same way the SQL
// adapter does, so chart-facing behaviour can be asserted without a database.
func (f *RouteStore) AggregateCheckTimeseries(_ context.Context, projectID int64, routeID *int64, since time.Time, bucketSeconds, maxBuckets int) ([]models.MetricPoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	if maxBuckets <= 0 {
		maxBuckets = 500
	}
	type agg struct {
		checks, failures, latencySum, latencyMax int
	}
	buckets := map[int64]*agg{}
	for id, checks := range f.checks {
		if routeID != nil && id != *routeID {
			continue
		}
		for _, c := range checks {
			if c.ProjectID != projectID || c.CheckedAt.Before(since) {
				continue
			}
			key := c.CheckedAt.Unix() / int64(bucketSeconds) * int64(bucketSeconds)
			b := buckets[key]
			if b == nil {
				b = &agg{}
				buckets[key] = b
			}
			b.checks++
			if c.Status == "down" {
				b.failures++
			}
			b.latencySum += c.LatencyMS
			if c.LatencyMS > b.latencyMax {
				b.latencyMax = c.LatencyMS
			}
		}
	}
	keys := make([]int64, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	out := make([]models.MetricPoint, 0, len(keys))
	for _, k := range keys {
		b := buckets[k]
		point := models.MetricPoint{
			BucketStart: time.Unix(k, 0).UTC(), Checks: b.checks, Failures: b.failures,
			AvgLatencyMS: b.latencySum / b.checks, MaxLatencyMS: b.latencyMax,
		}
		point.UptimePct = float64(b.checks-b.failures) / float64(b.checks) * 100
		out = append(out, point)
	}
	if len(out) > maxBuckets {
		out = out[:maxBuckets]
	}
	return out, nil
}

func (f *RouteStore) AggregateRouteMetrics(context.Context, time.Time) error { return nil }
func (f *RouteStore) AggregateProjectMetrics(context.Context) error          { return nil }
func (f *RouteStore) PruneRouteChecks(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

// ---------------------------------------------------------------- incidents

type RouteIncidentStore struct {
	mu     sync.Mutex
	nextID int64
	byID   map[int64]models.RouteIncident
	// Openings and Resolves count transitions, which is what the incident
	// open/resolve rule tests assert on.
	Openings int
	Resolves int
}

func NewRouteIncidentStore() *RouteIncidentStore {
	return &RouteIncidentStore{byID: map[int64]models.RouteIncident{}}
}

func (f *RouteIncidentStore) GetOpenRouteIncident(_ context.Context, routeID int64) (*models.RouteIncident, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, inc := range f.byID {
		if inc.RouteID == routeID && inc.State == "open" {
			copied := inc
			return &copied, nil
		}
	}
	return nil, nil
}

func (f *RouteIncidentStore) CreateRouteIncident(_ context.Context, routeID, projectID int64, reason string, startedAt time.Time) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.Openings++
	f.byID[f.nextID] = models.RouteIncident{
		ID: f.nextID, RouteID: routeID, ProjectID: projectID, State: "open",
		StartedAt: startedAt, LastFailureReason: reason, FailureCount: 1,
	}
	return f.nextID, nil
}

func (f *RouteIncidentStore) ResolveRouteIncident(_ context.Context, incidentID int64, resolvedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	inc, ok := f.byID[incidentID]
	if !ok {
		return nil
	}
	at := resolvedAt
	inc.State = "resolved"
	inc.ResolvedAt = &at
	f.byID[incidentID] = inc
	f.Resolves++
	return nil
}

func (f *RouteIncidentStore) ListRouteIncidents(_ context.Context, projectID int64, routeID *int64, state string, limit, offset int) ([]models.RouteIncident, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []models.RouteIncident{}
	for _, inc := range f.byID {
		if inc.ProjectID != projectID {
			continue
		}
		if routeID != nil && inc.RouteID != *routeID {
			continue
		}
		if state != "" && inc.State != state {
			continue
		}
		out = append(out, inc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return pageSlice(out, limit, offset), nil
}

// OpenCount reports how many incidents are currently open.
func (f *RouteIncidentStore) OpenCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, inc := range f.byID {
		if inc.State == "open" {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------- imports

type ImportStore struct {
	mu     sync.Mutex
	nextID int64
	byID   map[int64]models.ImportJob
}

func NewImportStore() *ImportStore { return &ImportStore{byID: map[int64]models.ImportJob{}} }

func (f *ImportStore) CreateImportJob(_ context.Context, job models.ImportJob) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	job.ID = f.nextID
	job.CreatedAt = time.Now().UTC()
	f.byID[job.ID] = job
	return job.ID, nil
}

func (f *ImportStore) GetImportJob(_ context.Context, id int64) (*models.ImportJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if job, ok := f.byID[id]; ok {
		copied := job
		return &copied, nil
	}
	return nil, nil
}

func (f *ImportStore) UpdateImportJob(_ context.Context, job models.ImportJob) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[job.ID] = job
	return nil
}

// ---------------------------------------------------------------- outbox

// OutboxStore records the alert events the service emits.
type OutboxStore struct {
	mu     sync.Mutex
	events []string
	keys   []string
}

func (f *OutboxStore) AddEvent(_ context.Context, eventType string, _ int64, dedupeKey string, _ []byte, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, eventType)
	f.keys = append(f.keys, dedupeKey)
	return nil
}
func (f *OutboxStore) FetchPending(context.Context, int) ([]models.OutboxEvent, error) {
	return nil, nil
}
func (f *OutboxStore) MarkProcessed(context.Context, int64) error      { return nil }
func (f *OutboxStore) MarkFailed(context.Context, int64, string) error { return nil }

// EventTypes returns the emitted event types in order.
func (f *OutboxStore) EventTypes() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.events))
	copy(out, f.events)
	return out
}

// DedupeKeys returns the emitted dedupe keys in order.
func (f *OutboxStore) DedupeKeys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.keys))
	copy(out, f.keys)
	return out
}

// ---------------------------------------------------------------- legacy

// LegacyStore satisfies the pre-existing website-monitoring ports. The
// project tests do not exercise them, but Service requires them.
type LegacyStore struct{}

func (LegacyStore) Create(context.Context, models.Website) (int64, error) { return 0, nil }
func (LegacyStore) GetByID(context.Context, int64) (*models.Website, error) {
	return nil, nil
}
func (LegacyStore) List(context.Context, int, int) ([]models.Website, error) {
	return nil, nil
}
func (LegacyStore) Delete(context.Context, int64) error { return nil }
func (LegacyStore) ListDue(context.Context, time.Time, int, int64) ([]models.Website, error) {
	return nil, nil
}
func (LegacyStore) MarkChecked(context.Context, int64, string, int, int, time.Time, time.Time) error {
	return nil
}
func (LegacyStore) RecordCheck(context.Context, int64, string, int, int, string, time.Time) error {
	return nil
}
func (LegacyStore) ListChecks(context.Context, *int64, int) ([]models.WebsiteCheck, error) {
	return nil, nil
}
func (LegacyStore) MarkHeartbeat(context.Context, int64, time.Time, time.Time) error { return nil }
func (LegacyStore) GetOpenIncident(context.Context, int64) (*models.Incident, error) {
	return nil, nil
}
func (LegacyStore) CreateIncident(context.Context, int64, string, time.Time) (int64, error) {
	return 0, nil
}
func (LegacyStore) ResolveIncident(context.Context, int64, time.Time) error { return nil }
func (LegacyStore) ListIncidents(context.Context, *int64, string, int, int) ([]models.Incident, error) {
	return nil, nil
}
func (LegacyStore) CreateMaintenanceWindow(context.Context, models.MaintenanceWindow) (int64, error) {
	return 0, nil
}
func (LegacyStore) IsWebsiteMuted(context.Context, int64, time.Time) (bool, error) {
	return false, nil
}
func (LegacyStore) CreateStatusPage(context.Context, models.StatusPage) (int64, error) {
	return 0, nil
}
func (LegacyStore) ListStatusPages(context.Context, int, int) ([]models.StatusPage, error) {
	return nil, nil
}
func (LegacyStore) GetStatusPageBySlug(context.Context, string) (*models.StatusPage, error) {
	return nil, nil
}
func (LegacyStore) ListWebsitesByStatusPage(context.Context, int64) ([]models.Website, error) {
	return nil, nil
}
func (LegacyStore) ListAlertChannels(context.Context) ([]models.AlertChannel, error) {
	return nil, nil
}
func (LegacyStore) CreateAlertChannel(context.Context, models.AlertChannel) (int64, error) {
	return 0, nil
}

// Compile-time proof the fakes satisfy the real ports.
var (
	_ ports.UserStore          = (*UserStore)(nil)
	_ ports.AuthTokenStore     = (*AuthTokenStore)(nil)
	_ ports.ProjectStore       = (*ProjectStore)(nil)
	_ ports.RouteStore         = (*RouteStore)(nil)
	_ ports.RouteIncidentStore = (*RouteIncidentStore)(nil)
	_ ports.ImportStore        = (*ImportStore)(nil)
	_ ports.OutboxStore        = (*OutboxStore)(nil)
	_ ports.MonitorStore       = LegacyStore{}
	_ ports.IncidentStore      = LegacyStore{}
	_ ports.MaintenanceStore   = LegacyStore{}
	_ ports.StatusPageStore    = LegacyStore{}
	_ ports.AlertChannelStore  = LegacyStore{}
)
