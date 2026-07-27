package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"argus/internal/domain"
	"argus/internal/models"
	"argus/internal/observability"

	"golang.org/x/crypto/bcrypt"
)

type fakeUserStore struct {
	nextID int64
	users  map[int64]models.User
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{nextID: 1, users: map[int64]models.User{}}
}
func (f *fakeUserStore) CreateUser(_ context.Context, user models.User) (int64, error) {
	user.ID = f.nextID
	f.nextID++
	f.users[user.ID] = user
	return user.ID, nil
}
func (f *fakeUserStore) GetUserByEmail(_ context.Context, email string) (*models.User, error) {
	for _, user := range f.users {
		if user.Email == email {
			copy := user
			return &copy, nil
		}
	}
	return nil, nil
}
func (f *fakeUserStore) GetUserByID(_ context.Context, id int64) (*models.User, error) {
	user, ok := f.users[id]
	if !ok {
		return nil, nil
	}
	copy := user
	return &copy, nil
}

type fakeTokenStore struct {
	nextID int64
	tokens map[string]models.AuthToken
}

func newFakeTokenStore() *fakeTokenStore {
	return &fakeTokenStore{nextID: 1, tokens: map[string]models.AuthToken{}}
}
func (f *fakeTokenStore) CreateToken(_ context.Context, token models.AuthToken) (int64, error) {
	token.ID = f.nextID
	f.nextID++
	f.tokens[token.TokenHash] = token
	return token.ID, nil
}
func (f *fakeTokenStore) GetTokenByHash(_ context.Context, hash string) (*models.AuthToken, error) {
	token, ok := f.tokens[hash]
	if !ok {
		return nil, nil
	}
	copy := token
	return &copy, nil
}
func (f *fakeTokenStore) TouchToken(_ context.Context, id int64, usedAt time.Time) error {
	for hash, token := range f.tokens {
		if token.ID == id {
			token.LastUsedAt = &usedAt
			f.tokens[hash] = token
		}
	}
	return nil
}
func (f *fakeTokenStore) DeleteToken(_ context.Context, hash string) error {
	delete(f.tokens, hash)
	return nil
}

type fakeProjectStore struct {
	projects map[int64]models.Project
	members  map[string]models.ProjectMember
}

func newFakeProjectStore() *fakeProjectStore {
	return &fakeProjectStore{projects: map[int64]models.Project{}, members: map[string]models.ProjectMember{}}
}
func memberKey(projectID, userID int64) string { return fmt.Sprintf("%d:%d", projectID, userID) }
func (f *fakeProjectStore) CreateProject(_ context.Context, project models.Project, ownerUserID int64) (int64, error) {
	project.ID = int64(len(f.projects) + 1)
	project.OwnerUserID = ownerUserID
	f.projects[project.ID] = project
	f.members[memberKey(project.ID, ownerUserID)] = models.ProjectMember{ProjectID: project.ID, UserID: ownerUserID, Role: models.ProjectRoleOwner}
	return project.ID, nil
}
func (f *fakeProjectStore) UpdateProject(_ context.Context, project models.Project) error {
	f.projects[project.ID] = project
	return nil
}
func (f *fakeProjectStore) SetProjectStatus(_ context.Context, id int64, status string) error {
	project := f.projects[id]
	project.Status = status
	f.projects[id] = project
	return nil
}
func (f *fakeProjectStore) DeleteProject(_ context.Context, id int64) error {
	delete(f.projects, id)
	return nil
}
func (f *fakeProjectStore) GetProjectByID(_ context.Context, id int64) (*models.Project, error) {
	project, ok := f.projects[id]
	if !ok {
		return nil, nil
	}
	copy := project
	return &copy, nil
}
func (f *fakeProjectStore) ListProjects(_ context.Context, userID int64, _ models.ProjectFilter) ([]models.Project, int, error) {
	out := []models.Project{}
	for id, project := range f.projects {
		if _, ok := f.members[memberKey(id, userID)]; ok {
			out = append(out, project)
		}
	}
	return out, len(out), nil
}
func (f *fakeProjectStore) GetProjectMember(_ context.Context, projectID, userID int64) (*models.ProjectMember, error) {
	member, ok := f.members[memberKey(projectID, userID)]
	if !ok {
		return nil, nil
	}
	copy := member
	return &copy, nil
}
func (f *fakeProjectStore) AddProjectMember(_ context.Context, member models.ProjectMember) error {
	f.members[memberKey(member.ProjectID, member.UserID)] = member
	return nil
}

type fakeRouteStore struct {
	nextID int64
	routes map[int64]models.APIRoute
	checks []models.RouteCheck
}

func newFakeRouteStore() *fakeRouteStore {
	return &fakeRouteStore{nextID: 1, routes: map[int64]models.APIRoute{}}
}

func (f *fakeRouteStore) CreateRoute(_ context.Context, route models.APIRoute) (int64, error) {
	for _, existing := range f.routes {
		if existing.ProjectID == route.ProjectID && existing.Method == route.Method && existing.Path == route.Path {
			return 0, domain.ErrDuplicateRoute
		}
	}
	route.ID = f.nextID
	f.nextID++
	f.routes[route.ID] = route
	return route.ID, nil
}
func (f *fakeRouteStore) BulkCreateRoutes(ctx context.Context, routes []models.APIRoute) (int, error) {
	for _, route := range routes {
		if _, err := f.CreateRoute(ctx, route); err != nil {
			return 0, err
		}
	}
	return len(routes), nil
}
func (f *fakeRouteStore) UpdateRoute(_ context.Context, route models.APIRoute) error {
	f.routes[route.ID] = route
	return nil
}
func (f *fakeRouteStore) UpdateRouteImportedMetadata(_ context.Context, route models.APIRoute) error {
	existing := f.routes[route.ID]
	existing.Name, existing.Summary, existing.Description = route.Name, route.Summary, route.Description
	existing.Tags, existing.Deprecated = route.Tags, route.Deprecated
	existing.Parameters, existing.RequestBody = route.Parameters, route.RequestBody
	existing.Responses, existing.Security = route.Responses, route.Security
	existing.SpecHash, existing.BaseURL = route.SpecHash, route.BaseURL
	f.routes[route.ID] = existing
	return nil
}
func (f *fakeRouteStore) SetRouteEnabled(_ context.Context, id int64, enabled bool) error {
	route := f.routes[id]
	route.Enabled = enabled
	if !enabled {
		route.Status = domain.RouteStatusDisabled
	}
	f.routes[id] = route
	return nil
}
func (f *fakeRouteStore) DeleteRoute(_ context.Context, id int64) error {
	delete(f.routes, id)
	return nil
}
func (f *fakeRouteStore) BulkDeleteRoutes(_ context.Context, projectID int64, ids []int64) (int64, error) {
	var count int64
	for _, id := range ids {
		if route, ok := f.routes[id]; ok && route.ProjectID == projectID {
			delete(f.routes, id)
			count++
		}
	}
	return count, nil
}
func (f *fakeRouteStore) GetRouteByID(_ context.Context, id int64) (*models.APIRoute, error) {
	route, ok := f.routes[id]
	if !ok {
		return nil, nil
	}
	copy := route
	return &copy, nil
}
func (f *fakeRouteStore) GetRouteByMethodPath(_ context.Context, projectID int64, method, path string) (*models.APIRoute, error) {
	for _, route := range f.routes {
		if route.ProjectID == projectID && route.Method == method && route.Path == path {
			copy := route
			return &copy, nil
		}
	}
	return nil, nil
}
func (f *fakeRouteStore) ListRoutes(_ context.Context, filter models.RouteFilter) ([]models.APIRoute, int, error) {
	out := []models.APIRoute{}
	for _, route := range f.routes {
		if route.ProjectID != filter.ProjectID ||
			(filter.Search != "" && !strings.Contains(strings.ToLower(route.Path+" "+route.Summary), strings.ToLower(filter.Search))) ||
			(filter.Method != "" && route.Method != filter.Method) ||
			(filter.Status != "" && route.Status != filter.Status) ||
			(filter.Enabled != nil && route.Enabled != *filter.Enabled) {
			continue
		}
		out = append(out, route)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	total := len(out)
	start := filter.Offset
	if start > total {
		start = total
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	end := start + limit
	if end > total {
		end = total
	}
	return out[start:end], total, nil
}
func (f *fakeRouteStore) ListAllRouteKeys(_ context.Context, projectID int64) (map[string]int64, error) {
	out := map[string]int64{}
	for id, route := range f.routes {
		if route.ProjectID == projectID {
			out[route.Method+" "+route.Path] = id
		}
	}
	return out, nil
}
func (f *fakeRouteStore) ListRouteSpecHashes(_ context.Context, projectID int64) (map[int64]string, error) {
	out := map[int64]string{}
	for id, route := range f.routes {
		if route.ProjectID == projectID {
			out[id] = route.SpecHash
		}
	}
	return out, nil
}
func (f *fakeRouteStore) ListDueRoutes(context.Context, time.Time, int, int64) ([]models.APIRoute, error) {
	return nil, nil
}
func (f *fakeRouteStore) MarkRouteChecked(_ context.Context, id int64, _ string, statusCode, latencyMS int, reason string, failures, successes int, status string, checkedAt, nextCheckAt time.Time) error {
	route := f.routes[id]
	route.LastCheckedAt, route.LastStatusCode, route.LastLatencyMS = &checkedAt, statusCode, latencyMS
	route.LastFailureReason, route.ConsecutiveFailures, route.ConsecutiveSuccesses = reason, failures, successes
	route.Status, route.NextCheckAt = status, nextCheckAt
	f.routes[id] = route
	return nil
}
func (f *fakeRouteStore) RecordRouteCheck(_ context.Context, check models.RouteCheck) error {
	f.checks = append(f.checks, check)
	return nil
}
func (f *fakeRouteStore) ListRouteChecks(_ context.Context, routeID int64, limit, offset int) ([]models.RouteCheck, error) {
	out := []models.RouteCheck{}
	for _, check := range f.checks {
		if check.RouteID == routeID {
			out = append(out, check)
		}
	}
	if offset >= len(out) {
		return []models.RouteCheck{}, nil
	}
	out = out[offset:]
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}
func (f *fakeRouteStore) AggregateRouteMetrics(context.Context, time.Time) error { return nil }
func (f *fakeRouteStore) AggregateProjectMetrics(context.Context) error          { return nil }
func (f *fakeRouteStore) PruneRouteChecks(context.Context, time.Time, int) (int64, error) {
	return 0, nil
}

type fakeImportStore struct {
	nextID int64
	jobs   map[int64]models.ImportJob
}

func newFakeImportStore() *fakeImportStore {
	return &fakeImportStore{nextID: 1, jobs: map[int64]models.ImportJob{}}
}
func (f *fakeImportStore) CreateImportJob(_ context.Context, job models.ImportJob) (int64, error) {
	job.ID = f.nextID
	f.nextID++
	f.jobs[job.ID] = job
	return job.ID, nil
}
func (f *fakeImportStore) GetImportJob(_ context.Context, id int64) (*models.ImportJob, error) {
	job, ok := f.jobs[id]
	if !ok {
		return nil, nil
	}
	copy := job
	return &copy, nil
}
func (f *fakeImportStore) UpdateImportJob(_ context.Context, job models.ImportJob) error {
	f.jobs[job.ID] = job
	return nil
}

type fakeRouteIncidentStore struct {
	nextID    int64
	incidents map[int64]models.RouteIncident
	opens     int
	resolves  int
}

func newFakeRouteIncidentStore() *fakeRouteIncidentStore {
	return &fakeRouteIncidentStore{nextID: 1, incidents: map[int64]models.RouteIncident{}}
}
func (f *fakeRouteIncidentStore) GetOpenRouteIncident(_ context.Context, routeID int64) (*models.RouteIncident, error) {
	for _, incident := range f.incidents {
		if incident.RouteID == routeID && incident.State == "open" {
			copy := incident
			return &copy, nil
		}
	}
	return nil, nil
}
func (f *fakeRouteIncidentStore) CreateRouteIncident(_ context.Context, routeID, projectID int64, reason string, startedAt time.Time) (int64, error) {
	id := f.nextID
	f.nextID++
	f.incidents[id] = models.RouteIncident{ID: id, RouteID: routeID, ProjectID: projectID, State: "open", StartedAt: startedAt, LastFailureReason: reason}
	f.opens++
	return id, nil
}
func (f *fakeRouteIncidentStore) ResolveRouteIncident(_ context.Context, incidentID int64, resolvedAt time.Time) error {
	incident := f.incidents[incidentID]
	incident.State, incident.ResolvedAt = "resolved", &resolvedAt
	f.incidents[incidentID] = incident
	f.resolves++
	return nil
}
func (f *fakeRouteIncidentStore) ListRouteIncidents(context.Context, int64, *int64, string, int, int) ([]models.RouteIncident, error) {
	return nil, nil
}

type fakeOutboxStore struct{ events int }

func (f *fakeOutboxStore) AddEvent(context.Context, string, int64, string, []byte, time.Time) error {
	f.events++
	return nil
}
func (*fakeOutboxStore) FetchPending(context.Context, int) ([]models.OutboxEvent, error) {
	return nil, nil
}
func (*fakeOutboxStore) MarkProcessed(context.Context, int64) error      { return nil }
func (*fakeOutboxStore) MarkFailed(context.Context, int64, string) error { return nil }

func projectService(routes *fakeRouteStore, imports *fakeImportStore, incidents *fakeRouteIncidentStore, outbox *fakeOutboxStore) *Service {
	return NewService(nil, nil, nil, nil, nil, outbox, observability.NewLogStore(100), nil, nil, nil, routes, incidents, imports)
}

func TestLargeImportPreviewCommitAndSearch(t *testing.T) {
	t.Parallel()
	routes, imports := newFakeRouteStore(), newFakeImportStore()
	service := projectService(routes, imports, newFakeRouteIncidentStore(), &fakeOutboxStore{})
	project := models.Project{ID: 7, DefaultIntervalSeconds: 60, DefaultTimeoutMS: 3000, DefaultRetries: 1, FailureThreshold: 3, RecoverySuccessThreshold: 1}

	paths := map[string]any{}
	for i := 0; i < 520; i++ {
		paths[fmt.Sprintf("/resources/%03d", i)] = map[string]any{
			"get": map[string]any{"operationId": fmt.Sprintf("getResource%03d", i), "responses": map[string]any{"200": map[string]any{"description": "ok"}}},
		}
	}
	spec, err := json.Marshal(map[string]any{
		"openapi": "3.0.3",
		"info":    map[string]any{"title": "Large API", "version": "1"},
		"servers": []map[string]string{{"url": "https://api.example.com"}},
		"paths":   paths,
	})
	if err != nil {
		t.Fatal(err)
	}
	job, err := service.ValidateImport(context.Background(), ValidateImportInput{ProjectID: project.ID, UserID: 1, SourceType: models.ImportSourcePaste, Data: spec})
	if err != nil {
		t.Fatal(err)
	}
	if job.TotalParsed != 520 || len(job.Items) != 520 {
		t.Fatalf("expected 520 preview routes, got parsed=%d items=%d", job.TotalParsed, len(job.Items))
	}
	committed, err := service.CommitImport(context.Background(), project, job.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if committed.CreatedRoutes != 520 || len(routes.routes) != 520 {
		t.Fatalf("expected 520 created routes, result=%+v stored=%d", committed, len(routes.routes))
	}
	found, total, err := service.ListRoutes(context.Background(), models.RouteFilter{ProjectID: project.ID, Search: "resources/419", Limit: 25})
	if err != nil || total != 1 || len(found) != 1 {
		t.Fatalf("search failed: total=%d found=%d err=%v", total, len(found), err)
	}
}

func TestReimportPreservesMonitoringSettingsAndHandlesRemovedRoutes(t *testing.T) {
	t.Parallel()
	routes, imports := newFakeRouteStore(), newFakeImportStore()
	service := projectService(routes, imports, newFakeRouteIncidentStore(), &fakeOutboxStore{})
	project := models.Project{ID: 1, DefaultIntervalSeconds: 60, DefaultTimeoutMS: 2000, FailureThreshold: 3, RecoverySuccessThreshold: 1}
	id, _ := routes.CreateRoute(context.Background(), models.APIRoute{
		ProjectID: 1, Method: "GET", Path: "/pets", BaseURL: "https://old.example.com", Summary: "old",
		SpecHash: "old-hash", Source: "import", Enabled: true, MonitorIntervalSecs: 777, TimeoutMS: 9876,
		Retries: 4, ExpectedStatusRange: "201-204", FailureThreshold: 5, RecoverySuccesses: 2,
	})
	removedID, _ := routes.CreateRoute(context.Background(), models.APIRoute{ProjectID: 1, Method: "DELETE", Path: "/legacy", Enabled: true, SpecHash: "legacy"})
	spec := []byte(`{"openapi":"3.0.3","info":{"title":"Pets","version":"2"},"servers":[{"url":"https://new.example.com"}],"paths":{"/pets":{"get":{"summary":"new","responses":{"200":{"description":"ok"}}}}}}`)

	job, err := service.ValidateImport(context.Background(), ValidateImportInput{ProjectID: 1, UserID: 1, SourceType: models.ImportSourcePaste, Data: spec})
	if err != nil {
		t.Fatal(err)
	}
	selections := map[string]models.ImportCommitSelection{
		"GET /pets":      {Key: "GET /pets", Selected: true},
		"DELETE /legacy": {Key: "DELETE /legacy", Selected: true},
	}
	result, err := service.CommitImport(context.Background(), project, job.ID, selections)
	if err != nil {
		t.Fatal(err)
	}
	if result.UpdatedRoutes != 1 || result.RemovedRoutes != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	updated := routes.routes[id]
	if updated.Summary != "new" || updated.BaseURL != "https://new.example.com" {
		t.Fatalf("import metadata was not updated: %+v", updated)
	}
	if updated.MonitorIntervalSecs != 777 || updated.TimeoutMS != 9876 || updated.Retries != 4 ||
		updated.ExpectedStatusRange != "201-204" || updated.FailureThreshold != 5 || updated.RecoverySuccesses != 2 {
		t.Fatalf("monitoring configuration was overwritten: %+v", updated)
	}
	if routes.routes[removedID].Enabled {
		t.Fatal("explicitly selected removed route should be disabled")
	}
}

func TestProcessRouteCheckCreatesOneIncidentAndResolvesAfterRecovery(t *testing.T) {
	t.Parallel()
	routes, incidents, outbox := newFakeRouteStore(), newFakeRouteIncidentStore(), &fakeOutboxStore{}
	service := projectService(routes, newFakeImportStore(), incidents, outbox)
	id, _ := routes.CreateRoute(context.Background(), models.APIRoute{
		ProjectID: 1, Method: "GET", Path: "/health", Enabled: true, MonitorIntervalSecs: 30,
		FailureThreshold: 3, RecoverySuccesses: 2, Status: domain.RouteStatusUnknown,
	})
	now := time.Now().UTC()
	for i := 0; i < 4; i++ {
		route := routes.routes[id]
		if err := service.ProcessRouteCheckResult(context.Background(), route, "down", 503, 10, "unavailable", 1, now.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if incidents.opens != 1 || incidents.resolves != 0 || outbox.events != 1 {
		t.Fatalf("failure transition mismatch: opens=%d resolves=%d events=%d", incidents.opens, incidents.resolves, outbox.events)
	}
	if routes.routes[id].Status != domain.RouteStatusFailing {
		t.Fatalf("expected failing, got %s", routes.routes[id].Status)
	}
	for i := 0; i < 2; i++ {
		route := routes.routes[id]
		if err := service.ProcessRouteCheckResult(context.Background(), route, "up", 200, 8, "", 1, now.Add(time.Duration(10+i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if incidents.opens != 1 || incidents.resolves != 1 || outbox.events != 2 {
		t.Fatalf("recovery transition mismatch: opens=%d resolves=%d events=%d", incidents.opens, incidents.resolves, outbox.events)
	}
	if routes.routes[id].Status != domain.RouteStatusHealthy {
		t.Fatalf("expected healthy, got %s", routes.routes[id].Status)
	}
}

func TestBulkCreateReportsDuplicatesAndInvalidRows(t *testing.T) {
	t.Parallel()
	routes := newFakeRouteStore()
	service := projectService(routes, newFakeImportStore(), newFakeRouteIncidentStore(), &fakeOutboxStore{})
	project := models.Project{ID: 1, DefaultIntervalSeconds: 60, DefaultTimeoutMS: 1000, FailureThreshold: 3, RecoverySuccessThreshold: 1}
	_, _ = routes.CreateRoute(context.Background(), models.APIRoute{ProjectID: 1, Method: "GET", Path: "/existing"})
	result, err := service.BulkCreateRoutes(context.Background(), project, []RouteInput{
		{Method: "GET", Path: "/new", BaseURL: "https://api.example.com"},
		{Method: "GET", Path: "/existing", BaseURL: "https://api.example.com"},
		{Method: "INVALID", Path: "/bad", BaseURL: "https://api.example.com"},
		{Method: "GET", Path: "/new", BaseURL: "https://api.example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Created) != 1 || len(result.Failed) != 3 {
		t.Fatalf("unexpected partial result: %+v", result)
	}
}

func TestRegisterLoginAndTokenExpiry(t *testing.T) {
	t.Parallel()
	users, tokens := newFakeUserStore(), newFakeTokenStore()
	service := NewService(nil, nil, nil, nil, nil, nil, observability.NewLogStore(50), users, tokens, nil, nil, nil, nil)
	ctx := context.Background()

	registered, err := service.Register(ctx, " OWNER@Example.com ", "strong-password", "Owner")
	if err != nil || registered.Token == "" || registered.User.Email != "owner@example.com" {
		t.Fatalf("register failed: result=%+v err=%v", registered, err)
	}
	if _, err = service.Register(ctx, "owner@example.com", "strong-password", "Duplicate"); err != ErrEmailTaken {
		t.Fatalf("expected duplicate email error, got %v", err)
	}
	if _, err = service.Login(ctx, "owner@example.com", "wrong-password"); err != ErrInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	loggedIn, err := service.Login(ctx, "owner@example.com", "strong-password")
	if err != nil || loggedIn.Token == "" {
		t.Fatalf("login failed: %v", err)
	}
	authenticated, err := service.Authenticate(ctx, loggedIn.Token)
	if err != nil || authenticated.ID != registered.User.ID || authenticated.PasswordHash != "" {
		t.Fatalf("authenticate failed: user=%+v err=%v", authenticated, err)
	}
	hash := hashToken(loggedIn.Token)
	expired := tokens.tokens[hash]
	expired.ExpiresAt = time.Now().UTC().Add(-time.Minute)
	tokens.tokens[hash] = expired
	if _, err = service.Authenticate(ctx, loggedIn.Token); err != ErrInvalidToken {
		t.Fatalf("expected expired token rejection, got %v", err)
	}
}

func TestAuthorizeProjectRolesAndEnumerationResistance(t *testing.T) {
	t.Parallel()
	projects := newFakeProjectStore()
	projects.projects[9] = models.Project{ID: 9, Name: "Payments"}
	projects.members[memberKey(9, 1)] = models.ProjectMember{ProjectID: 9, UserID: 1, Role: models.ProjectRoleOwner}
	projects.members[memberKey(9, 2)] = models.ProjectMember{ProjectID: 9, UserID: 2, Role: models.ProjectRoleEditor}
	projects.members[memberKey(9, 3)] = models.ProjectMember{ProjectID: 9, UserID: 3, Role: models.ProjectRoleViewer}
	service := NewService(nil, nil, nil, nil, nil, nil, observability.NewLogStore(20), nil, nil, projects, nil, nil, nil)

	tests := []struct {
		name    string
		project int64
		user    int64
		minRole string
		wantErr error
	}{
		{name: "owner may write", project: 9, user: 1, minRole: models.ProjectRoleEditor},
		{name: "editor may write", project: 9, user: 2, minRole: models.ProjectRoleEditor},
		{name: "viewer may read", project: 9, user: 3, minRole: models.ProjectRoleViewer},
		{name: "viewer cannot write", project: 9, user: 3, minRole: models.ProjectRoleEditor, wantErr: ErrInsufficientRole},
		{name: "nonmember hidden", project: 9, user: 99, minRole: models.ProjectRoleViewer, wantErr: domain.ErrProjectNotFound},
		{name: "missing hidden identically", project: 404, user: 1, minRole: models.ProjectRoleViewer, wantErr: domain.ErrProjectNotFound},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			_, _, err := service.AuthorizeProject(context.Background(), test.project, test.user, test.minRole)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("expected %v, got %v", test.wantErr, err)
			}
		})
	}
}

func TestLoginAcceptsStoredBcryptHash(t *testing.T) {
	t.Parallel()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-horse"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	users, tokens := newFakeUserStore(), newFakeTokenStore()
	users.users[42] = models.User{ID: 42, Email: "user@example.com", PasswordHash: string(hash)}
	service := NewService(nil, nil, nil, nil, nil, nil, observability.NewLogStore(10), users, tokens, nil, nil, nil, nil)
	if _, err = service.Login(context.Background(), "user@example.com", "correct-horse"); err != nil {
		t.Fatalf("valid bcrypt credentials rejected: %v", err)
	}
}
