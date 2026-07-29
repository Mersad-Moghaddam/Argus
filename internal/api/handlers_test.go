package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"argus/internal/application"
	"argus/internal/models"
	"argus/internal/observability"
	"argus/internal/openapi"
	"argus/internal/platform/httpserver"
	"argus/internal/testsupport"

	"github.com/gofiber/fiber/v2"
)

// testAPI spins up the real Fiber application — same middleware, same body
// limit, same route wiring as production — backed by in-memory stores.
type testAPI struct {
	app     *fiber.App
	service *application.Service
	stores  *testsupport.Stores
}

const legacyAPIKey = "legacy-key-for-tests"

func newTestAPI(t *testing.T) *testAPI {
	t.Helper()
	s := testsupport.NewStores()
	service := application.NewService(s.Legacy, s.Legacy, s.Legacy, s.Legacy, s.Legacy, s.Outbox,
		observability.NewLogStore(100), s.Users, s.Tokens, s.PasswordRecovery, s.RecoveryDelivery, s.Projects, s.Routes, s.Incidents, s.Imports, s.TelemetryCredentials, s.TelemetryIngress, s.TelemetryMappings, s.SLOs, s.Heartbeats)
	service.SetPrivateAgentStore(s.PrivateAgents)
	service.SetProjectIncidentStore(s.ProjectIncidents)
	return &testAPI{
		app:     httpserver.NewFiberApp(service, observability.NewLogStore(100), legacyAPIKey),
		service: service,
		stores:  s,
	}
}

// do executes a request against the app. A nil body sends no payload; a
// non-nil body is JSON-encoded.
func (a *testAPI) do(t *testing.T, method, path, token string, body any) *http.Response {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encode body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.app.Test(req, -1)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp
}

func decode(t *testing.T, resp *http.Response, into any) {
	t.Helper()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if into == nil {
		return
	}
	if err = json.Unmarshal(raw, into); err != nil {
		t.Fatalf("decode %q: %v", string(raw), err)
	}
}

func bodyString(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(raw)
}

// register creates an account through the public API and returns its token.
func (a *testAPI) register(t *testing.T, email string) (int64, string) {
	t.Helper()
	resp := a.do(t, http.MethodPost, "/identity/register", "", map[string]string{
		"email": email, "password": "longenoughpassword", "name": email,
	})
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("register %s: expected 201, got %d (%s)", email, resp.StatusCode, bodyString(t, resp))
	}
	var out struct {
		User  models.User `json:"user"`
		Token string      `json:"token"`
	}
	decode(t, resp, &out)
	if out.Token == "" {
		t.Fatal("expected a token")
	}
	return out.User.ID, out.Token
}

func (a *testAPI) createProject(t *testing.T, token, name string) models.Project {
	t.Helper()
	resp := a.do(t, http.MethodPost, "/project/catalog", token, map[string]any{"name": name})
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("create project: expected 201, got %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	var project models.Project
	decode(t, resp, &project)
	return project
}

func TestPrivateAgentManagementIsProjectScopedAndRevokesCredentials(t *testing.T) {
	a := newTestAPI(t)
	_, ownerToken := a.register(t, "agent-owner@example.com")
	_, outsiderToken := a.register(t, "agent-outsider@example.com")
	project := a.createProject(t, ownerToken, "Agent API")
	resp := a.do(t, http.MethodGet, fmt.Sprintf("/environment/catalog/%d", project.ID), ownerToken, nil)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("list environments: %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	var environments struct {
		Items []models.ProjectEnvironment `json:"items"`
	}
	decode(t, resp, &environments)
	if len(environments.Items) == 0 {
		t.Fatal("project should have a default environment")
	}
	resp = a.do(t, http.MethodPost, fmt.Sprintf("/agent/catalog/%d", project.ID), ownerToken, map[string]any{"name": "private edge", "environmentId": environments.Items[0].ID})
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("create agent: %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	var issued models.IssuedPrivateAgent
	decode(t, resp, &issued)
	if issued.EnrollmentToken == "" {
		t.Fatal("expected one-time enrollment token")
	}
	resp = a.do(t, http.MethodGet, fmt.Sprintf("/agent/catalog/%d", project.ID), ownerToken, nil)
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("list agents: %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	var listed struct {
		Items []models.PrivateAgent `json:"items"`
	}
	decode(t, resp, &listed)
	if len(listed.Items) != 1 || listed.Items[0].TokenHash != nil {
		t.Fatalf("unsafe agent list: %+v", listed.Items)
	}
	resp = a.do(t, http.MethodPost, fmt.Sprintf("/agent/revoke/%d/%d", project.ID, issued.Agent.ID), outsiderToken, nil)
	if resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("outsider revoke: expected non-enumerating 404, got %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	resp = a.do(t, http.MethodPost, fmt.Sprintf("/agent/revoke/%d/%d", project.ID, issued.Agent.ID), ownerToken, nil)
	if resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("revoke agent: %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	resp = a.do(t, http.MethodPost, "/agent/heartbeat", issued.EnrollmentToken, map[string]string{"version": "1.0.0"})
	if resp.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("revoked heartbeat: expected 401, got %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
}

func TestProjectIncidentEndpointsAreTenantScopedAndRoleAware(t *testing.T) {
	a := newTestAPI(t)
	viewerID, viewerToken := a.register(t, "project-incident-viewer@example.com")
	_, ownerToken := a.register(t, "project-incident-owner@example.com")
	_, outsiderToken := a.register(t, "project-incident-outsider@example.com")
	project := a.createProject(t, ownerToken, "Project incident API")
	if err := a.stores.Projects.AddProjectMember(context.Background(), models.ProjectMember{ProjectID: project.ID, UserID: viewerID, Role: models.ProjectRoleViewer}); err != nil {
		t.Fatal(err)
	}
	incidentID, err := a.stores.ProjectIncidents.CreateProjectIncident(context.Background(), models.ProjectIncident{
		ProjectID: project.ID, Source: "private_agent", SourceKey: "agent:12", Title: "Private agent is offline", Evidence: `{"agentId":12}`, StartedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	path := fmt.Sprintf("/incident/catalog/%d", project.ID)
	response := a.do(t, http.MethodGet, path, viewerToken, nil)
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("viewer list incidents: %d (%s)", response.StatusCode, bodyString(t, response))
	}
	var listed struct {
		Items []models.ProjectIncident `json:"items"`
	}
	decode(t, response, &listed)
	if len(listed.Items) != 1 || listed.Items[0].Source != "private_agent" || listed.Items[0].Evidence != `{"agentId":12}` {
		t.Fatalf("unexpected incidents: %#v", listed.Items)
	}
	response = a.do(t, http.MethodPost, fmt.Sprintf("/incident/acknowledge/%d/%d", project.ID, incidentID), viewerToken, nil)
	if response.StatusCode != fiber.StatusForbidden {
		t.Fatalf("viewer must not acknowledge: %d (%s)", response.StatusCode, bodyString(t, response))
	}
	response = a.do(t, http.MethodGet, path, outsiderToken, nil)
	if response.StatusCode != fiber.StatusNotFound {
		t.Fatalf("outsider must not list incidents: %d (%s)", response.StatusCode, bodyString(t, response))
	}
	response = a.do(t, http.MethodPost, fmt.Sprintf("/incident/acknowledge/%d/%d", project.ID, incidentID), ownerToken, nil)
	if response.StatusCode != fiber.StatusNoContent {
		t.Fatalf("owner acknowledge: %d (%s)", response.StatusCode, bodyString(t, response))
	}
	items, err := a.stores.ProjectIncidents.ListProjectIncidents(context.Background(), project.ID, "acknowledged", 10, 0)
	if err != nil || len(items) != 1 || items[0].AcknowledgedAt == nil {
		t.Fatalf("acknowledged incident: %v %#v", err, items)
	}
}

func TestSLODefinitionEndpointsEnforceProjectRoles(t *testing.T) {
	a := newTestAPI(t)
	viewerID, viewerToken := a.register(t, "slo-viewer@example.com")
	_, ownerToken := a.register(t, "slo-owner@example.com")
	project := a.createProject(t, ownerToken, "SLO API")
	if err := a.stores.Projects.AddProjectMember(context.Background(), models.ProjectMember{ProjectID: project.ID, UserID: viewerID, Role: models.ProjectRoleViewer}); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"name": "Availability", "sliKind": "availability", "targetPercent": 99.9, "minEvents": 20}
	if response := a.do(t, http.MethodPost, fmt.Sprintf("/slo/catalog/%d", project.ID), viewerToken, payload); response.StatusCode != fiber.StatusForbidden {
		t.Fatalf("viewer must not create SLO, got %d (%s)", response.StatusCode, bodyString(t, response))
	} else {
		_ = response.Body.Close()
	}
	response := a.do(t, http.MethodPost, fmt.Sprintf("/slo/catalog/%d", project.ID), ownerToken, payload)
	if response.StatusCode != fiber.StatusCreated {
		t.Fatalf("create SLO: %d (%s)", response.StatusCode, bodyString(t, response))
	}
	var definition models.SLODefinition
	decode(t, response, &definition)
	if definition.Version != 1 || definition.ID == 0 {
		t.Fatalf("unexpected SLO response: %+v", definition)
	}
	if response = a.do(t, http.MethodGet, fmt.Sprintf("/slo/catalog/%d", project.ID), viewerToken, nil); response.StatusCode != fiber.StatusOK {
		t.Fatalf("viewer must list SLOs: %d", response.StatusCode)
	} else {
		_ = response.Body.Close()
	}
	if response = a.do(t, http.MethodGet, fmt.Sprintf("/slo/evaluations/%d/%d", project.ID, definition.ID), viewerToken, nil); response.StatusCode != fiber.StatusOK {
		t.Fatalf("viewer must list SLO evaluations: %d", response.StatusCode)
	} else {
		_ = response.Body.Close()
	}
}

// ------------------------------------------------------------ authentication

func TestAuthEndpoints(t *testing.T) {
	a := newTestAPI(t)

	t.Run("register validation errors are 400", func(t *testing.T) {
		resp := a.do(t, http.MethodPost, "/identity/register", "", map[string]string{"email": "nope", "password": "longenoughpassword"})
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("register then login", func(t *testing.T) {
		_, token := a.register(t, "api-user@example.com")
		resp := a.do(t, http.MethodPost, "/identity/login", "", map[string]string{
			"email": "api-user@example.com", "password": "longenoughpassword",
		})
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", resp.StatusCode, bodyString(t, resp))
		}
		body := bodyString(t, resp)
		if strings.Contains(body, "passwordHash") || strings.Contains(body, "longenoughpassword") {
			t.Fatalf("login response leaked credentials: %s", body)
		}

		me := a.do(t, http.MethodGet, "/identity/profile", token, nil)
		if me.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200 from /auth/me, got %d", me.StatusCode)
		}
	})

	t.Run("bad password is 401", func(t *testing.T) {
		a.register(t, "pw@example.com")
		resp := a.do(t, http.MethodPost, "/identity/login", "", map[string]string{"email": "pw@example.com", "password": "wrongwrongwrong"})
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("password recovery is generic and one time", func(t *testing.T) {
		a.register(t, "recovery-api@example.com")
		known := a.do(t, http.MethodPost, "/identity/recovery/request", "", map[string]string{"email": "recovery-api@example.com"})
		if known.StatusCode != fiber.StatusAccepted {
			t.Fatalf("known recovery request: %d (%s)", known.StatusCode, bodyString(t, known))
		}
		knownBody := bodyString(t, known)
		unknown := a.do(t, http.MethodPost, "/identity/recovery/request", "", map[string]string{"email": "unknown-recovery@example.com"})
		if unknown.StatusCode != fiber.StatusAccepted || bodyString(t, unknown) != knownBody {
			t.Fatal("recovery request must not reveal account existence")
		}
		token := a.stores.RecoveryDelivery.Token
		if token == "" || strings.Contains(knownBody, token) {
			t.Fatal("recovery token must be delivered out of band only")
		}
		complete := a.do(t, http.MethodPost, "/identity/recovery/complete", "", map[string]string{"token": token, "newPassword": "new-recovery-password"})
		if complete.StatusCode != fiber.StatusNoContent {
			t.Fatalf("complete recovery: %d (%s)", complete.StatusCode, bodyString(t, complete))
		}
		_ = complete.Body.Close()
		reused := a.do(t, http.MethodPost, "/identity/recovery/complete", "", map[string]string{"token": token, "newPassword": "another-password"})
		if reused.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("reused recovery token: %d", reused.StatusCode)
		}
		_ = reused.Body.Close()
	})

	t.Run("logout revokes the token", func(t *testing.T) {
		_, token := a.register(t, "logout@example.com")
		if resp := a.do(t, http.MethodPost, "/identity/logout", token, nil); resp.StatusCode != fiber.StatusNoContent {
			t.Fatalf("expected 204, got %d", resp.StatusCode)
		}
		if resp := a.do(t, http.MethodGet, "/project/catalog", token, nil); resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("a revoked token must be rejected, got %d", resp.StatusCode)
		}
	})

	t.Run("change password keeps current session and invalidates old credentials", func(t *testing.T) {
		_, token := a.register(t, "change-password@example.com")
		resp := a.do(t, http.MethodPost, "/identity/password", token, map[string]string{
			"currentPassword": "longenoughpassword",
			"newPassword":     "new-long-password",
		})
		if resp.StatusCode != fiber.StatusNoContent {
			t.Fatalf("expected 204, got %d (%s)", resp.StatusCode, bodyString(t, resp))
		}
		_ = resp.Body.Close()
		if resp := a.do(t, http.MethodGet, "/identity/profile", token, nil); resp.StatusCode != fiber.StatusOK {
			t.Fatalf("current session must remain valid, got %d", resp.StatusCode)
		} else {
			_ = resp.Body.Close()
		}
		if resp := a.do(t, http.MethodPost, "/identity/login", "", map[string]string{"email": "change-password@example.com", "password": "longenoughpassword"}); resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("old password must fail, got %d", resp.StatusCode)
		} else {
			_ = resp.Body.Close()
		}
	})
}

func TestBrowserSessionUsesHttpOnlyCookieAndCSRF(t *testing.T) {
	a := newTestAPI(t)
	resp := a.do(t, http.MethodPost, "/identity/register", "", map[string]string{
		"email": "cookie@example.com", "password": "longenoughpassword", "name": "Cookie user",
	})
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	cookies := resp.Cookies()
	_ = resp.Body.Close()
	var session, csrf *http.Cookie
	for _, cookie := range cookies {
		switch cookie.Name {
		case "argus_session":
			session = cookie
		case "argus_csrf":
			csrf = cookie
		}
	}
	if session == nil || !session.HttpOnly || session.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected HttpOnly SameSite=Lax session cookie, got %#v", session)
	}
	if csrf == nil || csrf.HttpOnly || csrf.Value == "" {
		t.Fatalf("expected readable CSRF cookie, got %#v", csrf)
	}

	me := httptest.NewRequest(http.MethodGet, "/identity/profile", nil)
	me.AddCookie(session)
	meResp, err := a.app.Test(me, -1)
	if err != nil {
		t.Fatalf("cookie /auth/me: %v", err)
	}
	if meResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected cookie auth to work, got %d", meResp.StatusCode)
	}
	_ = meResp.Body.Close()

	create := httptest.NewRequest(http.MethodPost, "/project/catalog", strings.NewReader(`{"name":"Cookie project"}`))
	create.Header.Set("Content-Type", "application/json")
	create.AddCookie(session)
	create.AddCookie(csrf)
	missingCSRF, err := a.app.Test(create, -1)
	if err != nil {
		t.Fatalf("missing csrf request: %v", err)
	}
	if missingCSRF.StatusCode != fiber.StatusForbidden {
		t.Fatalf("expected 403 without CSRF, got %d", missingCSRF.StatusCode)
	}
	_ = missingCSRF.Body.Close()

	create = httptest.NewRequest(http.MethodPost, "/project/catalog", strings.NewReader(`{"name":"Cookie project"}`))
	create.Header.Set("Content-Type", "application/json")
	create.Header.Set("X-CSRF-Token", csrf.Value)
	create.AddCookie(session)
	create.AddCookie(csrf)
	created, err := a.app.Test(create, -1)
	if err != nil {
		t.Fatalf("csrf request: %v", err)
	}
	if created.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201 with CSRF, got %d (%s)", created.StatusCode, bodyString(t, created))
	}
	_ = created.Body.Close()
}

func TestProjectRoutesRequireBearerToken(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "bearer@example.com")
	project := a.createProject(t, token, "Guarded")

	protected := []struct{ method, path string }{
		{http.MethodGet, "/project/catalog"},
		{http.MethodPost, "/project/catalog"},
		{http.MethodGet, fmt.Sprintf("/project/catalog/%d", project.ID)},
		{http.MethodPut, fmt.Sprintf("/project/catalog/%d", project.ID)},
		{http.MethodDelete, fmt.Sprintf("/project/catalog/%d", project.ID)},
		{http.MethodPost, fmt.Sprintf("/project/archive/%d", project.ID)},
		{http.MethodGet, fmt.Sprintf("/route/catalog/%d", project.ID)},
		{http.MethodPost, fmt.Sprintf("/route/catalog/%d", project.ID)},
		{http.MethodPost, fmt.Sprintf("/route/bulk/%d", project.ID)},
		{http.MethodPost, fmt.Sprintf("/route/removal/%d", project.ID)},
		{http.MethodGet, fmt.Sprintf("/route/incidents/%d", project.ID)},
		{http.MethodPost, fmt.Sprintf("/import/validation/%d", project.ID)},
		{http.MethodGet, fmt.Sprintf("/telemetry/credentials/%d", project.ID)},
		{http.MethodGet, fmt.Sprintf("/telemetry/ingress/%d", project.ID)},
		{http.MethodPost, fmt.Sprintf("/telemetry/credentials/%d", project.ID)},
	}

	for _, tc := range protected {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			for _, header := range []string{"", "garbage", "Bearer ", "Bearer not-a-real-token"} {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				if header != "" {
					req.Header.Set("Authorization", header)
				}
				resp, err := a.app.Test(req, -1)
				if err != nil {
					t.Fatalf("request: %v", err)
				}
				if resp.StatusCode != fiber.StatusUnauthorized {
					t.Fatalf("Authorization=%q: expected 401, got %d", header, resp.StatusCode)
				}
				_ = resp.Body.Close()
			}
		})
	}
}

func TestControlPlaneRoutesUseFamilyAndPurposeWithoutAPIPrefix(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "taxonomy@example.com")
	project := a.createProject(t, token, "Taxonomy")
	if response := a.do(t, http.MethodGet, fmt.Sprintf("/project/catalog/%d", project.ID), token, nil); response.StatusCode != fiber.StatusOK {
		t.Fatalf("family/purpose route must work: %d (%s)", response.StatusCode, bodyString(t, response))
	} else {
		_ = response.Body.Close()
	}
	if response := a.do(t, http.MethodGet, fmt.Sprintf("/api/project/catalog/%d", project.ID), token, nil); response.StatusCode != fiber.StatusNotFound {
		t.Fatalf("removed API prefix must not be mounted: %d (%s)", response.StatusCode, bodyString(t, response))
	} else {
		_ = response.Body.Close()
	}
}

func TestTelemetryCredentialEndpoints(t *testing.T) {
	a := newTestAPI(t)
	ownerID, ownerToken := a.register(t, "telemetry-owner@example.com")
	viewerID, viewerToken := a.register(t, "telemetry-viewer@example.com")
	_ = ownerID
	project := a.createProject(t, ownerToken, "Telemetry")
	if err := a.stores.Projects.AddProjectMember(context.Background(), models.ProjectMember{ProjectID: project.ID, UserID: viewerID, Role: models.ProjectRoleViewer}); err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	environments, err := a.stores.Projects.ListProjectEnvironments(context.Background(), project.ID)
	if err != nil || len(environments) != 1 {
		t.Fatalf("default environment: %v %+v", err, environments)
	}
	base := fmt.Sprintf("/telemetry/credentials/%d", project.ID)
	created := a.do(t, http.MethodPost, base, ownerToken, map[string]any{
		"name": "production collector", "environmentId": environments[0].ID, "expiresInDays": 30,
	})
	if created.StatusCode != fiber.StatusCreated {
		t.Fatalf("create credential: %d (%s)", created.StatusCode, bodyString(t, created))
	}
	var issued models.IssuedTelemetryCredential
	decode(t, created, &issued)
	if !strings.HasPrefix(issued.Token, "argus_otlp_") || issued.Credential.TokenPrefix == "" || len(issued.Credential.TokenHash) != 0 {
		t.Fatalf("expected a one-time token without hash leakage: %+v", issued)
	}
	if _, err := a.service.AuthenticateTelemetryCredential(context.Background(), issued.Token); err != nil {
		t.Fatalf("created secret must authenticate: %v", err)
	}
	if resp := a.do(t, http.MethodGet, fmt.Sprintf("/telemetry/ingress/%d", project.ID), viewerToken, nil); resp.StatusCode != fiber.StatusOK {
		t.Fatalf("viewer diagnostics: expected 200, got %d", resp.StatusCode)
	} else {
		_ = resp.Body.Close()
	}

	if resp := a.do(t, http.MethodPost, base, viewerToken, map[string]any{"name": "no", "environmentId": environments[0].ID}); resp.StatusCode != fiber.StatusForbidden {
		t.Fatalf("viewer create: expected 403, got %d", resp.StatusCode)
	} else {
		_ = resp.Body.Close()
	}
	listed := a.do(t, http.MethodGet, base, viewerToken, nil)
	if listed.StatusCode != fiber.StatusOK {
		t.Fatalf("viewer list: expected 200, got %d", listed.StatusCode)
	}
	if body := bodyString(t, listed); strings.Contains(body, issued.Token) || strings.Contains(body, "tokenHash") {
		t.Fatalf("credential listing leaked secret material: %s", body)
	}

	rotated := a.do(t, http.MethodPost, fmt.Sprintf("/telemetry/rotate/%d/%d", project.ID, issued.Credential.ID), ownerToken, map[string]any{})
	if rotated.StatusCode != fiber.StatusCreated {
		t.Fatalf("rotate: %d (%s)", rotated.StatusCode, bodyString(t, rotated))
	}
	var replacement models.IssuedTelemetryCredential
	decode(t, rotated, &replacement)
	if _, err := a.service.AuthenticateTelemetryCredential(context.Background(), issued.Token); !errors.Is(err, application.ErrTelemetryCredentialNotFound) {
		t.Fatalf("old token must not authenticate after rotation: %v", err)
	}

	revoked := a.do(t, http.MethodPost, fmt.Sprintf("/telemetry/revoke/%d/%d", project.ID, replacement.Credential.ID), ownerToken, nil)
	if revoked.StatusCode != fiber.StatusNoContent {
		t.Fatalf("revoke: expected 204, got %d (%s)", revoked.StatusCode, bodyString(t, revoked))
	}
	_ = revoked.Body.Close()
	if _, err := a.service.AuthenticateTelemetryCredential(context.Background(), replacement.Token); !errors.Is(err, application.ErrTelemetryCredentialNotFound) {
		t.Fatalf("revoked token must not authenticate: %v", err)
	}
}

func TestHeartbeatEndpointsAreScopedAndDoNotLeakSecrets(t *testing.T) {
	a := newTestAPI(t)
	_, ownerToken := a.register(t, "heartbeat-owner@example.com")
	_, viewerToken := a.register(t, "heartbeat-viewer@example.com")
	project := a.createProject(t, ownerToken, "Heartbeat project")
	environments, _ := a.stores.Projects.ListProjectEnvironments(context.Background(), project.ID)
	created := a.do(t, http.MethodPost, fmt.Sprintf("/heartbeat/catalog/%d", project.ID), ownerToken, map[string]any{"name": "nightly backup", "environmentId": environments[0].ID, "expectedIntervalSeconds": 300, "gracePeriodSeconds": 120})
	if created.StatusCode != fiber.StatusCreated {
		t.Fatalf("create heartbeat: %d (%s)", created.StatusCode, bodyString(t, created))
	}
	var issued models.IssuedHeartbeatMonitor
	decode(t, created, &issued)
	if !strings.HasPrefix(issued.Token, "argus_hb_") || len(issued.Monitor.TokenHash) != 0 {
		t.Fatalf("unsafe issued heartbeat: %+v", issued)
	}
	list := a.do(t, http.MethodGet, fmt.Sprintf("/heartbeat/catalog/%d", project.ID), viewerToken, nil)
	if list.StatusCode != fiber.StatusNotFound {
		t.Fatalf("nonmember list: %d (%s)", list.StatusCode, bodyString(t, list))
	}
	req := httptest.NewRequest(http.MethodPost, "/heartbeat/ping", strings.NewReader(`{"outcome":"success"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+issued.Token)
	req.Header.Set("Idempotency-Key", "nightly-2026-07-29-0001")
	resp, err := a.app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusAccepted {
		t.Fatalf("ping: %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	_ = resp.Body.Close()
	requestReplay := httptest.NewRequest(http.MethodPost, "/heartbeat/ping", strings.NewReader(`{"outcome":"success"}`))
	requestReplay.Header.Set("Content-Type", "application/json")
	requestReplay.Header.Set("Authorization", "Bearer "+issued.Token)
	requestReplay.Header.Set("Idempotency-Key", "nightly-2026-07-29-0001")
	replay, err := a.app.Test(requestReplay, -1)
	if err != nil {
		t.Fatal(err)
	}
	if replay.StatusCode != fiber.StatusAccepted || !strings.Contains(bodyString(t, replay), `"accepted":false`) {
		t.Fatal("duplicate heartbeat must be accepted without refreshing liveness")
	}
	if denied := a.do(t, http.MethodPost, fmt.Sprintf("/heartbeat/revoke/%d/%d", project.ID, issued.Monitor.ID), viewerToken, nil); denied.StatusCode != fiber.StatusNotFound {
		t.Fatalf("nonmember revoke: %d", denied.StatusCode)
	} else {
		_ = denied.Body.Close()
	}
}

// ------------------------------------------------------------ authorization

func TestProjectAuthorizationMatrix(t *testing.T) {
	a := newTestAPI(t)
	ctx := context.Background()

	ownerID, ownerToken := a.register(t, "owner@example.com")
	editorID, editorToken := a.register(t, "editor@example.com")
	viewerID, viewerToken := a.register(t, "viewer@example.com")
	_, strangerToken := a.register(t, "stranger@example.com")
	_ = ownerID

	project := a.createProject(t, ownerToken, "Shared")
	if err := a.stores.Projects.AddProjectMember(ctx, models.ProjectMember{ProjectID: project.ID, UserID: editorID, Role: models.ProjectRoleEditor}); err != nil {
		t.Fatalf("add editor: %v", err)
	}
	if err := a.stores.Projects.AddProjectMember(ctx, models.ProjectMember{ProjectID: project.ID, UserID: viewerID, Role: models.ProjectRoleViewer}); err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	base := fmt.Sprintf("/project/catalog/%d", project.ID)
	routes := fmt.Sprintf("/route/catalog/%d", project.ID)
	incidents := fmt.Sprintf("/route/incidents/%d", project.ID)
	bulkRemoval := fmt.Sprintf("/route/removal/%d", project.ID)
	importValidation := fmt.Sprintf("/import/validation/%d", project.ID)
	archive := fmt.Sprintf("/project/archive/%d", project.ID)
	restore := fmt.Sprintf("/project/restore/%d", project.ID)
	routeBody := map[string]any{"method": "GET", "path": "/authz-probe", "baseUrl": "https://api.example.com"}

	cases := []struct {
		name   string
		method string
		path   string
		token  string
		body   any
		want   int
	}{
		{"viewer can read the project", http.MethodGet, base, viewerToken, nil, fiber.StatusOK},
		{"viewer can list routes", http.MethodGet, routes, viewerToken, nil, fiber.StatusOK},
		{"viewer can list incidents", http.MethodGet, incidents, viewerToken, nil, fiber.StatusOK},
		{"viewer cannot create routes", http.MethodPost, routes, viewerToken, routeBody, fiber.StatusForbidden},
		{"viewer cannot bulk delete", http.MethodPost, bulkRemoval, viewerToken, map[string]any{"ids": []int64{1}}, fiber.StatusForbidden},
		{"viewer cannot update the project", http.MethodPut, base, viewerToken, map[string]any{"name": "x"}, fiber.StatusForbidden},
		{"viewer cannot import", http.MethodPost, importValidation, viewerToken, map[string]any{"spec": "{}"}, fiber.StatusForbidden},
		{"viewer cannot archive", http.MethodPost, archive, viewerToken, nil, fiber.StatusForbidden},
		{"viewer cannot delete", http.MethodDelete, base, viewerToken, nil, fiber.StatusForbidden},

		{"editor can create routes", http.MethodPost, routes, editorToken, routeBody, fiber.StatusCreated},
		{"editor can update the project", http.MethodPut, base, editorToken, map[string]any{"name": "Shared v2"}, fiber.StatusOK},
		{"editor cannot archive", http.MethodPost, archive, editorToken, nil, fiber.StatusForbidden},
		{"editor cannot delete the project", http.MethodDelete, base, editorToken, nil, fiber.StatusForbidden},

		{"owner can archive", http.MethodPost, archive, ownerToken, nil, fiber.StatusNoContent},
		{"owner can unarchive", http.MethodPost, restore, ownerToken, nil, fiber.StatusNoContent},

		// A non-member must never learn that the project exists.
		{"stranger gets 404 reading", http.MethodGet, base, strangerToken, nil, fiber.StatusNotFound},
		{"stranger gets 404 writing", http.MethodPost, routes, strangerToken, routeBody, fiber.StatusNotFound},
		{"stranger gets 404 deleting", http.MethodDelete, base, strangerToken, nil, fiber.StatusNotFound},
		{"stranger gets 404 listing routes", http.MethodGet, routes, strangerToken, nil, fiber.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := a.do(t, tc.method, tc.path, tc.token, tc.body)
			if resp.StatusCode != tc.want {
				t.Fatalf("expected %d, got %d (%s)", tc.want, resp.StatusCode, bodyString(t, resp))
			}
			_ = resp.Body.Close()
		})
	}
}

// TestNonexistentAndForbiddenProjectsAreIndistinguishable is the anti-
// enumeration guarantee: the response for "not a member" must be
// byte-identical to the response for "does not exist".
func TestNonexistentAndForbiddenProjectsAreIndistinguishable(t *testing.T) {
	a := newTestAPI(t)
	_, ownerToken := a.register(t, "enum-owner@example.com")
	_, strangerToken := a.register(t, "enum-stranger@example.com")
	project := a.createProject(t, ownerToken, "Hidden")

	forbidden := a.do(t, http.MethodGet, fmt.Sprintf("/project/catalog/%d", project.ID), strangerToken, nil)
	missing := a.do(t, http.MethodGet, fmt.Sprintf("/project/catalog/%d", project.ID+9999), strangerToken, nil)

	if forbidden.StatusCode != fiber.StatusNotFound || missing.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected both to be 404, got %d and %d", forbidden.StatusCode, missing.StatusCode)
	}
	if got, want := bodyString(t, forbidden), bodyString(t, missing); got != want {
		t.Fatalf("responses must be identical: %q vs %q", got, want)
	}
}

func TestInvalidProjectIDIsRejected(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "badid@example.com")
	for _, id := range []string{"abc", "0", "-1"} {
		resp := a.do(t, http.MethodGet, "/project/catalog/"+id, token, nil)
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("project id %q: expected 400, got %d", id, resp.StatusCode)
		}
		_ = resp.Body.Close()
	}
}

// ------------------------------------------------------------ routes

func TestCreateRouteEndpoint(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "routes@example.com")
	project := a.createProject(t, token, "Routes")
	base := fmt.Sprintf("/route/catalog/%d", project.ID)

	t.Run("creates and returns 201", func(t *testing.T) {
		resp := a.do(t, http.MethodPost, base, token, map[string]any{
			"method": "get", "path": "pets/", "baseUrl": "https://api.example.com/",
		})
		if resp.StatusCode != fiber.StatusCreated {
			t.Fatalf("expected 201, got %d (%s)", resp.StatusCode, bodyString(t, resp))
		}
		var route models.APIRoute
		decode(t, resp, &route)
		if route.Method != "GET" || route.Path != "/pets" {
			t.Fatalf("normalization not applied: %+v", route)
		}
	})

	t.Run("duplicate is 400 with a useful message", func(t *testing.T) {
		resp := a.do(t, http.MethodPost, base, token, map[string]any{
			"method": "GET", "path": "/pets", "baseUrl": "https://api.example.com",
		})
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
		if body := bodyString(t, resp); !strings.Contains(body, "already exists") {
			t.Fatalf("expected a duplicate-route message, got %s", body)
		}
	})

	t.Run("invalid method is 400", func(t *testing.T) {
		resp := a.do(t, http.MethodPost, base, token, map[string]any{
			"method": "TELEPORT", "path": "/x", "baseUrl": "https://api.example.com",
		})
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("malformed JSON is 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, base, strings.NewReader("{not json"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := a.app.Test(req, -1)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})
}

func TestEndpointNormalizationPreview(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "normalization-preview@example.com")
	project := a.createProject(t, token, "Preview")
	resp := a.do(t, http.MethodPost, fmt.Sprintf("/route/normalization/%d", project.ID), token, map[string]any{
		"method": " get ", "baseUrl": "HTTPS://EXAMPLE.COM:443/api/../", "routeTemplate": "v1/%70ets/{petId}", "intervalSeconds": 300,
	})
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	var preview struct {
		Valid     bool                                                      `json:"valid"`
		Canonical struct{ Method, BaseURL, RouteTemplate, Identity string } `json:"canonical"`
		Safety    struct {
			ProbeDefault, Traffic   string
			EstimatedRequestsPerDay int `json:"estimatedRequestsPerDay"`
		} `json:"safety"`
	}
	decode(t, resp, &preview)
	if !preview.Valid || preview.Canonical.Method != "GET" || preview.Canonical.BaseURL != "https://example.com" || preview.Canonical.RouteTemplate != "/v1/pets/{petId}" {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if preview.Safety.ProbeDefault != "disabled" || preview.Safety.Traffic != "catalog_only" || preview.Safety.EstimatedRequestsPerDay != 288 {
		t.Fatalf("unexpected safety preview: %+v", preview.Safety)
	}

	bad := a.do(t, http.MethodPost, fmt.Sprintf("/route/normalization/%d", project.ID), token, map[string]any{"method": "GET", "baseUrl": "example.com", "routeTemplate": "/x"})
	if bad.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400, got %d", bad.StatusCode)
	}
	var failure struct{ Code, Field string }
	decode(t, bad, &failure)
	if failure.Code != "absolute_url_required" || failure.Field != "baseUrl" {
		t.Fatalf("unexpected validation error: %+v", failure)
	}
}

func TestRouteResponsesRedactSecretHeaders(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "redact@example.com")
	project := a.createProject(t, token, "Redaction")

	resp := a.do(t, http.MethodPost, fmt.Sprintf("/route/catalog/%d", project.ID), token, map[string]any{
		"method": "GET", "path": "/secured", "baseUrl": "https://api.example.com",
		"headers": `{"Authorization":"Bearer super-secret","X-Trace":"visible"}`,
	})
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	var created models.APIRoute
	decode(t, resp, &created)
	if strings.Contains(created.Headers, "super-secret") {
		t.Fatalf("the create response leaked a secret: %s", created.Headers)
	}
	if !strings.Contains(created.Headers, "visible") {
		t.Fatalf("non-sensitive headers should still be visible: %s", created.Headers)
	}

	for _, path := range []string{
		fmt.Sprintf("/route/catalog/%d/%d", project.ID, created.ID),
		fmt.Sprintf("/route/catalog/%d", project.ID),
	} {
		body := bodyString(t, a.do(t, http.MethodGet, path, token, nil))
		if strings.Contains(body, "super-secret") {
			t.Fatalf("%s leaked a secret: %s", path, body)
		}
	}

	// The stored value must still be the real secret so checks work.
	stored, err := a.stores.Routes.GetRouteByID(context.Background(), created.ID)
	if err != nil || stored == nil {
		t.Fatalf("load stored route: %v", err)
	}
	if !strings.Contains(stored.Headers, "super-secret") {
		t.Fatalf("redaction must be a read-time concern, storage keeps the real value: %s", stored.Headers)
	}
}

func TestBulkCreateRoutesEndpoint(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "bulk@example.com")
	project := a.createProject(t, token, "Bulk")
	path := fmt.Sprintf("/route/bulk/%d", project.ID)

	t.Run("reports per-row outcomes", func(t *testing.T) {
		resp := a.do(t, http.MethodPost, path, token, map[string]any{"routes": []map[string]any{
			{"method": "GET", "path": "/a", "baseUrl": "https://api.example.com"},
			{"method": "WRONG", "path": "/b", "baseUrl": "https://api.example.com"},
			{"method": "POST", "path": "/c", "baseUrl": "https://api.example.com"},
			{"method": "GET", "path": "/a", "baseUrl": "https://api.example.com"},
		}})
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", resp.StatusCode, bodyString(t, resp))
		}
		var result application.BulkCreateResult
		decode(t, resp, &result)
		if len(result.Created) != 2 {
			t.Fatalf("expected 2 created, got %d", len(result.Created))
		}
		if len(result.Failed) != 2 {
			t.Fatalf("expected 2 failures, got %+v", result.Failed)
		}
		for _, f := range result.Failed {
			if f.Error == "" || f.Route == "" {
				t.Fatalf("every failure must name its row and reason: %+v", f)
			}
		}
	})

	t.Run("empty batch is 400", func(t *testing.T) {
		resp := a.do(t, http.MethodPost, path, token, map[string]any{"routes": []any{}})
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("oversized batch is rejected before any work", func(t *testing.T) {
		rows := make([]map[string]any, 5001)
		for i := range rows {
			rows[i] = map[string]any{"method": "GET", "path": fmt.Sprintf("/r%d", i), "baseUrl": "https://api.example.com"}
		}
		resp := a.do(t, http.MethodPost, path, token, map[string]any{"routes": rows})
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
		if body := bodyString(t, resp); !strings.Contains(body, "too many routes") {
			t.Fatalf("expected an explicit limit message, got %s", body)
		}
	})
}

// TestRouteFromAnotherProjectIsNotReachable covers the cross-project guard:
// being an authorized member of project A must not grant access to a route ID
// that belongs to project B.
func TestRouteFromAnotherProjectIsNotReachable(t *testing.T) {
	a := newTestAPI(t)
	_, tokenA := a.register(t, "tenant-a@example.com")
	_, tokenB := a.register(t, "tenant-b@example.com")
	projectA := a.createProject(t, tokenA, "Tenant A")
	projectB := a.createProject(t, tokenB, "Tenant B")

	resp := a.do(t, http.MethodPost, fmt.Sprintf("/route/catalog/%d", projectB.ID), tokenB, map[string]any{
		"method": "GET", "path": "/private", "baseUrl": "https://b.example.com",
	})
	var victim models.APIRoute
	decode(t, resp, &victim)
	if victim.ID == 0 {
		t.Fatal("failed to seed the victim route")
	}

	for _, tc := range []struct{ method, suffix string }{
		{http.MethodGet, ""},
		{http.MethodPut, ""},
		{http.MethodDelete, ""},
		{http.MethodPost, "/disable"},
		{http.MethodGet, "/checks"},
	} {
		path := fmt.Sprintf("/route/catalog/%d/%d%s", projectA.ID, victim.ID, tc.suffix)
		var body any
		if tc.method == http.MethodPut {
			body = map[string]any{"method": "GET", "path": "/private"}
		}
		got := a.do(t, tc.method, path, tokenA, body)
		if got.StatusCode != fiber.StatusNotFound {
			t.Fatalf("%s %s: expected 404, got %d", tc.method, path, got.StatusCode)
		}
		_ = got.Body.Close()
	}

	// And it is genuinely untouched.
	survivor, err := a.stores.Routes.GetRouteByID(context.Background(), victim.ID)
	if err != nil || survivor == nil {
		t.Fatal("the victim route must still exist")
	}
}

func TestBulkDeleteIsProjectScopedOverHTTP(t *testing.T) {
	a := newTestAPI(t)
	_, tokenA := a.register(t, "bd-a@example.com")
	_, tokenB := a.register(t, "bd-b@example.com")
	projectA := a.createProject(t, tokenA, "BD A")
	projectB := a.createProject(t, tokenB, "BD B")

	var mine, theirs models.APIRoute
	decode(t, a.do(t, http.MethodPost, fmt.Sprintf("/route/catalog/%d", projectA.ID), tokenA,
		map[string]any{"method": "GET", "path": "/mine", "baseUrl": "https://a.example"}), &mine)
	decode(t, a.do(t, http.MethodPost, fmt.Sprintf("/route/catalog/%d", projectB.ID), tokenB,
		map[string]any{"method": "GET", "path": "/theirs", "baseUrl": "https://b.example"}), &theirs)

	resp := a.do(t, http.MethodPost, fmt.Sprintf("/route/removal/%d", projectA.ID), tokenA,
		map[string]any{"ids": []int64{mine.ID, theirs.ID}})
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out struct {
		Deleted int64 `json:"deleted"`
	}
	decode(t, resp, &out)
	if out.Deleted != 1 {
		t.Fatalf("expected exactly 1 deletion, got %d", out.Deleted)
	}
	if survivor, _ := a.stores.Routes.GetRouteByID(context.Background(), theirs.ID); survivor == nil {
		t.Fatal("a bulk delete must never reach another project's routes")
	}

	empty := a.do(t, http.MethodPost, fmt.Sprintf("/route/removal/%d", projectA.ID), tokenA, map[string]any{"ids": []int64{}})
	if empty.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for an empty id list, got %d", empty.StatusCode)
	}
}

func TestRouteLifecycleEndpoints(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "lifecycle@example.com")
	project := a.createProject(t, token, "Lifecycle")
	base := fmt.Sprintf("/route/catalog/%d", project.ID)

	var route models.APIRoute
	decode(t, a.do(t, http.MethodPost, base, token, map[string]any{
		"method": "GET", "path": "/thing", "baseUrl": "https://api.example.com", "summary": "Thing",
	}), &route)

	one := fmt.Sprintf("%s/%d", base, route.ID)

	if resp := a.do(t, http.MethodPost, fmt.Sprintf("/route/disable/%d/%d", project.ID, route.ID), token, nil); resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("disable: expected 204, got %d", resp.StatusCode)
	}
	var afterDisable models.APIRoute
	decode(t, a.do(t, http.MethodGet, one, token, nil), &afterDisable)
	if afterDisable.Enabled || afterDisable.Status != "disabled" {
		t.Fatalf("expected a disabled route, got %+v", afterDisable)
	}

	if resp := a.do(t, http.MethodPost, fmt.Sprintf("/route/enable/%d/%d", project.ID, route.ID), token, nil); resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("enable: expected 204, got %d", resp.StatusCode)
	}

	var updated models.APIRoute
	decode(t, a.do(t, http.MethodPut, one, token, map[string]any{"summary": "Renamed", "timeoutMs": 1234}), &updated)
	if updated.Summary != "Renamed" || updated.TimeoutMS != 1234 {
		t.Fatalf("update did not apply: %+v", updated)
	}

	checks := a.do(t, http.MethodGet, fmt.Sprintf("/route/checks/%d/%d", project.ID, route.ID), token, nil)
	if checks.StatusCode != fiber.StatusOK {
		t.Fatalf("checks: expected 200, got %d", checks.StatusCode)
	}

	if resp := a.do(t, http.MethodDelete, one, token, nil); resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("delete: expected 204, got %d", resp.StatusCode)
	}
	if resp := a.do(t, http.MethodGet, one, token, nil); resp.StatusCode != fiber.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", resp.StatusCode)
	}
}

func TestListRoutesQueryParameters(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "query@example.com")
	project := a.createProject(t, token, "Query")
	base := fmt.Sprintf("/route/catalog/%d", project.ID)

	for _, r := range []map[string]any{
		{"method": "GET", "path": "/pets", "baseUrl": "https://a.example", "tags": []string{"pets"}},
		{"method": "POST", "path": "/pets", "baseUrl": "https://a.example", "tags": []string{"pets"}},
		{"method": "GET", "path": "/orders", "baseUrl": "https://a.example", "tags": []string{"orders"}, "deprecated": true},
	} {
		if resp := a.do(t, http.MethodPost, base, token, r); resp.StatusCode != fiber.StatusCreated {
			t.Fatalf("seed route: %d (%s)", resp.StatusCode, bodyString(t, resp))
		}
	}

	type listResponse struct {
		Items []models.APIRoute `json:"items"`
		Total int               `json:"total"`
	}
	cases := []struct {
		query string
		want  int
	}{
		{"", 3},
		{"?search=pets", 2},
		{"?method=get", 2}, // lower-case must be accepted
		{"?tag=orders", 1},
		{"?deprecated=true", 1},
		{"?deprecated=false", 2},
		{"?enabled=true", 3},
		{"?enabled=false", 0},
		{"?status=unknown", 3},
		{"?limit=1", 3}, // total is unpaged
	}
	for _, tc := range cases {
		t.Run("list"+tc.query, func(t *testing.T) {
			var out listResponse
			decode(t, a.do(t, http.MethodGet, base+tc.query, token, nil), &out)
			if out.Total != tc.want {
				t.Fatalf("expected total %d, got %d", tc.want, out.Total)
			}
		})
	}

	var page listResponse
	decode(t, a.do(t, http.MethodGet, base+"?limit=1&offset=1&sortBy=path", token, nil), &page)
	if len(page.Items) != 1 {
		t.Fatalf("expected a page of 1, got %d", len(page.Items))
	}
}

func TestMetricsTimeseriesEndpoint(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "metrics@example.com")
	_, otherToken := a.register(t, "metrics-other@example.com")
	project := a.createProject(t, token, "Metrics")
	base := fmt.Sprintf("/route/metrics/%d", project.ID)

	var route models.APIRoute
	decode(t, a.do(t, http.MethodPost, fmt.Sprintf("/route/catalog/%d", project.ID), token,
		map[string]any{"method": "GET", "path": "/charted", "baseUrl": "https://a.example"}), &route)

	if err := a.stores.Routes.RecordRouteCheck(context.Background(), models.RouteCheck{
		RouteID: route.ID, ProjectID: project.ID, Status: "up", StatusCode: 200,
		LatencyMS: 80, CheckedAt: time.Now().UTC().Add(-2 * time.Minute),
	}); err != nil {
		t.Fatalf("seed check: %v", err)
	}

	t.Run("returns a bounded bucketed series", func(t *testing.T) {
		var series application.MetricsTimeseries
		decode(t, a.do(t, http.MethodGet, base+"?range=1h", token, nil), &series)
		if series.Range != "1h" || series.BucketSeconds != 120 {
			t.Fatalf("unexpected window: %+v", series.TimeseriesWindow)
		}
		if len(series.Points) != 1 || series.Points[0].Checks != 1 {
			t.Fatalf("expected one bucket with one check, got %+v", series.Points)
		}
	})

	t.Run("an unknown range falls back instead of failing", func(t *testing.T) {
		var series application.MetricsTimeseries
		decode(t, a.do(t, http.MethodGet, base+"?range=forever", token, nil), &series)
		if series.Range != application.DefaultTimeseriesRange {
			t.Fatalf("expected the default range, got %q", series.Range)
		}
	})

	t.Run("routeId narrows the series", func(t *testing.T) {
		resp := a.do(t, http.MethodGet, fmt.Sprintf("%s?range=1h&routeId=%d", base, route.ID), token, nil)
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		_ = resp.Body.Close()
	})

	t.Run("a bad routeId is 400", func(t *testing.T) {
		resp := a.do(t, http.MethodGet, base+"?routeId=abc", token, nil)
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
		_ = resp.Body.Close()
	})

	t.Run("a routeId from another project is 404", func(t *testing.T) {
		otherProject := a.createProject(t, otherToken, "Other Metrics")
		var foreign models.APIRoute
		decode(t, a.do(t, http.MethodPost, fmt.Sprintf("/route/catalog/%d", otherProject.ID), otherToken,
			map[string]any{"method": "GET", "path": "/foreign", "baseUrl": "https://b.example"}), &foreign)

		resp := a.do(t, http.MethodGet, fmt.Sprintf("%s?routeId=%d", base, foreign.ID), token, nil)
		if resp.StatusCode != fiber.StatusNotFound {
			t.Fatalf("expected 404 for a cross-project routeId, got %d", resp.StatusCode)
		}
		_ = resp.Body.Close()
	})

	t.Run("a non-member gets 404", func(t *testing.T) {
		resp := a.do(t, http.MethodGet, base, otherToken, nil)
		if resp.StatusCode != fiber.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
		_ = resp.Body.Close()
	})

	t.Run("no token is 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, base, nil)
		resp, err := a.app.Test(req, -1)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
}

// ------------------------------------------------------------ imports

func multipartSpec(t *testing.T, filename, content, baseURLOverride string) (string, io.Reader) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err = io.WriteString(part, content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if baseURLOverride != "" {
		if err = writer.WriteField("baseUrlOverride", baseURLOverride); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	return writer.FormDataContentType(), &buf
}

func (a *testAPI) upload(t *testing.T, path, token, filename, content, baseURLOverride string) *http.Response {
	t.Helper()
	contentType, body := multipartSpec(t, filename, content, baseURLOverride)
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := a.app.Test(req, -1)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	return resp
}

const importSpec = `{
  "openapi": "3.0.0",
  "info": {"title": "Imported", "version": "1.0"},
  "servers": [{"url": "https://imported.example.com"}],
  "paths": {
    "/alpha": {"get": {"operationId": "getAlpha", "summary": "Alpha"}},
    "/beta": {"post": {"operationId": "postBeta", "summary": "Beta"}}
  }
}`

func TestImportValidateAndCommitOverHTTP(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "import@example.com")
	project := a.createProject(t, token, "Import")
	validatePath := fmt.Sprintf("/import/validation/%d", project.ID)

	t.Run("file upload", func(t *testing.T) {
		resp := a.upload(t, validatePath, token, "spec.json", importSpec, "")
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", resp.StatusCode, bodyString(t, resp))
		}
		var job models.ImportJob
		decode(t, resp, &job)
		if job.SourceType != models.ImportSourceFile {
			t.Fatalf("expected sourceType=file, got %q", job.SourceType)
		}
		if len(job.Items) != 2 {
			t.Fatalf("expected 2 preview items, got %d", len(job.Items))
		}

		// Fetch the job back.
		var fetched models.ImportJob
		decode(t, a.do(t, http.MethodGet, fmt.Sprintf("/import/job/%d/%d", project.ID, job.ID), token, nil), &fetched)
		if fetched.ID != job.ID {
			t.Fatalf("expected job %d, got %d", job.ID, fetched.ID)
		}

		// Commit it.
		commitPath := fmt.Sprintf("/import/commit/%d/%d", project.ID, job.ID)
		var committed models.ImportJob
		commitResp := a.do(t, http.MethodPost, commitPath, token, map[string]any{"selections": []map[string]any{
			{"key": "GET /alpha", "selected": true},
			{"key": "POST /beta", "selected": true},
		}})
		if commitResp.StatusCode != fiber.StatusOK {
			t.Fatalf("commit: expected 200, got %d (%s)", commitResp.StatusCode, bodyString(t, commitResp))
		}
		decode(t, commitResp, &committed)
		if committed.CreatedRoutes != 2 || committed.Status != models.ImportStatusCommitted {
			t.Fatalf("unexpected commit result: %+v", committed)
		}

		// Replaying the commit is a conflict, not a silent double-import.
		replay := a.do(t, http.MethodPost, commitPath, token, map[string]any{"selections": []any{}})
		if replay.StatusCode != fiber.StatusConflict {
			t.Fatalf("expected 409 on replay, got %d", replay.StatusCode)
		}
	})

	t.Run("pasted spec", func(t *testing.T) {
		resp := a.do(t, http.MethodPost, validatePath, token, map[string]string{
			"spec": importSpec, "baseUrlOverride": "https://staging.example.com",
		})
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d (%s)", resp.StatusCode, bodyString(t, resp))
		}
		var job models.ImportJob
		decode(t, resp, &job)
		if job.SourceType != models.ImportSourcePaste {
			t.Fatalf("expected sourceType=paste, got %q", job.SourceType)
		}
		for _, item := range job.Items {
			if item.BaseURL != "https://staging.example.com" {
				t.Fatalf("base URL override ignored: %q", item.BaseURL)
			}
		}
	})
}

func TestImportRejectsBadUploads(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "badimport@example.com")
	project := a.createProject(t, token, "Bad Import")
	path := fmt.Sprintf("/import/validation/%d", project.ID)

	t.Run("malformed spec is 400 with a useful error", func(t *testing.T) {
		resp := a.upload(t, path, token, "spec.json", "{ this is not a spec", "")
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
		body := bodyString(t, resp)
		if !strings.Contains(body, "parse") && !strings.Contains(body, "recognizable") {
			t.Fatalf("expected an explanatory error, got %s", body)
		}
	})

	t.Run("valid YAML that is not a spec is 400", func(t *testing.T) {
		resp := a.upload(t, path, token, "spec.yaml", "hello: world\nlist:\n  - a\n  - b\n", "")
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected 400, got %d (%s)", resp.StatusCode, bodyString(t, resp))
		}
	})

	t.Run("empty upload is 400", func(t *testing.T) {
		resp := a.do(t, http.MethodPost, path, token, map[string]string{"spec": ""})
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("oversized upload is 413", func(t *testing.T) {
		oversized := strings.Repeat("a", openapi.MaxDocumentBytes+1)
		resp := a.upload(t, path, token, "huge.json", oversized, "")
		if resp.StatusCode != fiber.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413, got %d (%s)", resp.StatusCode, bodyString(t, resp))
		}
	})

	t.Run("oversized paste is 413", func(t *testing.T) {
		resp := a.do(t, http.MethodPost, path, token, map[string]string{
			"spec": strings.Repeat("b", openapi.MaxDocumentBytes+1),
		})
		if resp.StatusCode != fiber.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413, got %d (%s)", resp.StatusCode, bodyString(t, resp))
		}
	})

	t.Run("unknown job is 404", func(t *testing.T) {
		resp := a.do(t, http.MethodGet, fmt.Sprintf("/import/job/%d/424242", project.ID), token, nil)
		if resp.StatusCode != fiber.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})
}

// TestImportFiveHundredRoutesOverHTTP is acceptance criterion 1 driven end to
// end through the real HTTP surface.
func TestImportFiveHundredRoutesOverHTTP(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "large@example.com")
	project := a.createProject(t, token, "Large Import")

	const resources = 150
	const expected = resources * 4
	spec := largeSpec(resources)

	resp := a.upload(t, fmt.Sprintf("/import/validation/%d", project.ID), token, "large.json", spec, "")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("validate: expected 200, got %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	var job models.ImportJob
	decode(t, resp, &job)
	if job.TotalParsed != expected {
		t.Fatalf("expected %d parsed routes, got %d", expected, job.TotalParsed)
	}

	selections := make([]map[string]any, 0, len(job.Items))
	for _, item := range job.Items {
		selections = append(selections, map[string]any{"key": item.Key, "selected": true})
	}
	commit := a.do(t, http.MethodPost, fmt.Sprintf("/import/commit/%d/%d", project.ID, job.ID), token,
		map[string]any{"selections": selections})
	if commit.StatusCode != fiber.StatusOK {
		t.Fatalf("commit: expected 200, got %d (%s)", commit.StatusCode, bodyString(t, commit))
	}
	var committed models.ImportJob
	decode(t, commit, &committed)
	if committed.CreatedRoutes != expected {
		t.Fatalf("expected %d created, got %d", expected, committed.CreatedRoutes)
	}

	var list struct {
		Items []models.APIRoute `json:"items"`
		Total int               `json:"total"`
	}
	decode(t, a.do(t, http.MethodGet, fmt.Sprintf("/route/catalog/%d?limit=25", project.ID), token, nil), &list)
	if list.Total != expected {
		t.Fatalf("expected %d routes listed, got %d", expected, list.Total)
	}
	if len(list.Items) != 25 {
		t.Fatalf("expected the page size to be honoured, got %d rows", len(list.Items))
	}

	var searched struct {
		Total int `json:"total"`
	}
	decode(t, a.do(t, http.MethodGet, fmt.Sprintf("/route/catalog/%d?search=resource042", project.ID), token, nil), &searched)
	if searched.Total != 4 {
		t.Fatalf("expected 4 search hits, got %d", searched.Total)
	}
}

func largeSpec(resources int) string {
	var b strings.Builder
	b.WriteString(`{"openapi":"3.0.0","info":{"title":"Large API","version":"1.0"},`)
	b.WriteString(`"servers":[{"url":"https://large.example.com/v1"}],"paths":{`)
	for i := 0; i < resources; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		name := fmt.Sprintf("resource%03d", i)
		fmt.Fprintf(&b, `"/%s":{"get":{"operationId":"list_%s","summary":"List %s"},`, name, name, name)
		fmt.Fprintf(&b, `"post":{"operationId":"create_%s","summary":"Create %s"}},`, name, name)
		fmt.Fprintf(&b, `"/%s/{id}":{"get":{"operationId":"get_%s","summary":"Get %s",`, name, name, name)
		b.WriteString(`"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"integer"}}]},`)
		fmt.Fprintf(&b, `"delete":{"operationId":"delete_%s","summary":"Delete %s"}}`, name, name)
	}
	b.WriteString(`}}`)
	return b.String()
}

// ------------------------------------------------------------ backward compatibility

// TestLegacyAPIKeyRoutesAreUnchanged guards the requirement that the existing
// single-tenant monitoring API keeps its own X-API-Key protection and is not
// affected by the new bearer-token scheme.
func TestLegacyAPIKeyRoutesAreUnchanged(t *testing.T) {
	a := newTestAPI(t)
	_, bearerToken := a.register(t, "legacy@example.com")

	t.Run("no key is 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/monitor/websites", nil)
		resp, err := a.app.Test(req, -1)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("a project bearer token does not unlock the legacy API", func(t *testing.T) {
		resp := a.do(t, http.MethodGet, "/monitor/websites", bearerToken, nil)
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})

	t.Run("the API key still works", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/monitor/websites", nil)
		req.Header.Set("X-API-Key", legacyAPIKey)
		resp, err := a.app.Test(req, -1)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("the API key does not unlock the project API", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/project/catalog", nil)
		req.Header.Set("X-API-Key", legacyAPIKey)
		resp, err := a.app.Test(req, -1)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != fiber.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
}

func TestProjectListingEndpoint(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "listing@example.com")
	_, otherToken := a.register(t, "listing-other@example.com")

	active := a.createProject(t, token, "Active Project")
	archived := a.createProject(t, token, "Archived Project")
	a.createProject(t, otherToken, "Not Mine")

	if resp := a.do(t, http.MethodPost, fmt.Sprintf("/project/archive/%d", archived.ID), token, nil); resp.StatusCode != fiber.StatusNoContent {
		t.Fatalf("archive: got %d", resp.StatusCode)
	}

	type listResponse struct {
		Items []models.Project `json:"items"`
		Total int              `json:"total"`
	}
	var all listResponse
	decode(t, a.do(t, http.MethodGet, "/project/catalog", token, nil), &all)
	if all.Total != 2 {
		t.Fatalf("expected only the caller's 2 projects, got %d", all.Total)
	}

	var activeOnly listResponse
	decode(t, a.do(t, http.MethodGet, "/project/catalog?status=active", token, nil), &activeOnly)
	if activeOnly.Total != 1 || activeOnly.Items[0].ID != active.ID {
		t.Fatalf("status filter failed: %+v", activeOnly)
	}

	var searched listResponse
	decode(t, a.do(t, http.MethodGet, "/project/catalog?search=archived", token, nil), &searched)
	if searched.Total != 1 || searched.Items[0].ID != archived.ID {
		t.Fatalf("search filter failed: %+v", searched)
	}

	if resp := a.do(t, http.MethodPost, "/project/catalog", token, map[string]any{"name": "  "}); resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("expected 400 for a blank name, got %d", resp.StatusCode)
	}
}

func TestProjectEnvironmentEndpoints(t *testing.T) {
	a := newTestAPI(t)
	_, token := a.register(t, "environment-api@example.com")
	project := a.createProject(t, token, "Environment API")

	resp := a.do(t, http.MethodPost, fmt.Sprintf("/environment/catalog/%d", project.ID), token, map[string]string{
		"name": "staging", "baseUrl": "HTTPS://API.Example.com:443/v1/",
	})
	if resp.StatusCode != fiber.StatusCreated {
		t.Fatalf("create environment: expected 201, got %d (%s)", resp.StatusCode, bodyString(t, resp))
	}
	var created models.ProjectEnvironment
	decode(t, resp, &created)
	if created.CanonicalBaseURL != "https://api.example.com/v1" || created.CanonicalOrigin != "https://api.example.com" {
		t.Fatalf("environment was not canonicalized: %+v", created)
	}

	var listed struct {
		Items []models.ProjectEnvironment `json:"items"`
	}
	decode(t, a.do(t, http.MethodGet, fmt.Sprintf("/environment/catalog/%d", project.ID), token, nil), &listed)
	if len(listed.Items) != 2 {
		t.Fatalf("environments = %d, want default production plus staging", len(listed.Items))
	}
}
