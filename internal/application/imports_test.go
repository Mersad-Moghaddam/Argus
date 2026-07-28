package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"argus/internal/domain"
	"argus/internal/models"
	"argus/internal/openapi"
	"argus/internal/testsupport"
)

const petstoreSpec = `{
  "openapi": "3.0.0",
  "info": {"title": "Petstore", "version": "1.0"},
  "servers": [{"url": "https://petstore.example.com/v1"}],
  "paths": {
    "/pets": {
      "get": {"operationId": "listPets", "summary": "List pets", "tags": ["pets"]},
      "post": {"operationId": "createPet", "summary": "Create a pet", "tags": ["pets"]}
    },
    "/pets/{petId}": {
      "get": {
        "operationId": "getPet", "summary": "Fetch a pet", "tags": ["pets"],
        "parameters": [{"name": "petId", "in": "path", "required": true, "schema": {"type": "integer"}}]
      },
      "delete": {"operationId": "deletePet", "summary": "Remove a pet", "deprecated": true, "tags": ["pets"]}
    }
  }
}`

func validate(t *testing.T, h *testHarness, project models.Project, spec string, baseOverride string) models.ImportJob {
	t.Helper()
	job, err := h.service.ValidateImport(context.Background(), ValidateImportInput{
		ProjectID: project.ID, UserID: 1, SourceType: models.ImportSourcePaste,
		Data: []byte(spec), BaseURLOverride: baseOverride,
	})
	if err != nil {
		t.Fatalf("validate import: %v", err)
	}
	return job
}

func itemsByKey(job models.ImportJob) map[string]models.ImportRouteItem {
	out := map[string]models.ImportRouteItem{}
	for _, item := range job.Items {
		out[item.Key] = item
	}
	return out
}

// commitAll commits a job accepting the backend's default selection for every
// item, which is what the wizard's "continue" button does.
func commitAll(t *testing.T, h *testHarness, project models.Project, job models.ImportJob) models.ImportJob {
	t.Helper()
	committed, err := h.service.CommitImport(context.Background(), project, job.ID, nil)
	if err != nil {
		t.Fatalf("commit import: %v", err)
	}
	return committed
}

func TestValidateImportParsesAndMarksEverythingNew(t *testing.T) {
	h := newTestHarness()
	project := seedProject(t, h)

	job := validate(t, h, project, petstoreSpec, "")
	if job.SpecFormat != openapi.FormatOpenAPI3 {
		t.Fatalf("expected openapi3, got %q", job.SpecFormat)
	}
	if job.Status != models.ImportStatusValidated {
		t.Fatalf("expected a validated job, got %q", job.Status)
	}
	if job.TotalParsed != 4 || len(job.Items) != 4 {
		t.Fatalf("expected 4 parsed operations, got %d parsed / %d items", job.TotalParsed, len(job.Items))
	}

	items := itemsByKey(job)
	for _, key := range []string{"GET /pets", "POST /pets", "GET /pets/{petId}", "DELETE /pets/{petId}"} {
		item, ok := items[key]
		if !ok {
			t.Fatalf("missing item %q", key)
		}
		if item.Action != models.ImportActionCreate {
			t.Fatalf("%s: expected action=create, got %q", key, item.Action)
		}
		if item.Conflict != models.ImportConflictNone {
			t.Fatalf("%s: expected no conflict, got %q", key, item.Conflict)
		}
		if !item.Selected {
			t.Fatalf("%s: new routes must be pre-selected", key)
		}
		if item.BaseURL != "https://petstore.example.com/v1" {
			t.Fatalf("%s: server URL not applied, got %q", key, item.BaseURL)
		}
	}
	if !items["DELETE /pets/{petId}"].Deprecated {
		t.Fatal("the deprecated flag must survive parsing")
	}
	if !strings.Contains(items["GET /pets/{petId}"].Parameters, "petId") {
		t.Fatalf("path parameters must be captured, got %q", items["GET /pets/{petId}"].Parameters)
	}

	// Nothing may be created before commit.
	if _, total, err := h.service.ListRoutes(context.Background(), models.RouteFilter{ProjectID: project.ID}); err != nil || total != 0 {
		t.Fatalf("validate must not mutate routes: %d routes exist (%v)", total, err)
	}
}

func TestValidateImportHonoursBaseURLOverride(t *testing.T) {
	h := newTestHarness()
	project := seedProject(t, h)
	job := validate(t, h, project, petstoreSpec, "https://staging.example.com/api/")
	for _, item := range job.Items {
		if item.BaseURL != "https://staging.example.com/api" {
			t.Fatalf("expected the override (trailing slash trimmed), got %q", item.BaseURL)
		}
	}
}

func TestValidateImportRejectsBadInput(t *testing.T) {
	h := newTestHarness()
	project := seedProject(t, h)
	ctx := context.Background()

	t.Run("malformed document", func(t *testing.T) {
		_, err := h.service.ValidateImport(ctx, ValidateImportInput{
			ProjectID: project.ID, SourceType: models.ImportSourcePaste, Data: []byte("{ this is not valid"),
		})
		if !errors.Is(err, openapi.ErrUnsupportedInput) && !errors.Is(err, openapi.ErrUnsupportedSpec) {
			t.Fatalf("expected a parse error, got %v", err)
		}
	})

	t.Run("valid JSON that is not a spec", func(t *testing.T) {
		_, err := h.service.ValidateImport(ctx, ValidateImportInput{
			ProjectID: project.ID, SourceType: models.ImportSourcePaste, Data: []byte(`{"hello":"world"}`),
		})
		if !errors.Is(err, openapi.ErrUnsupportedSpec) {
			t.Fatalf("expected ErrUnsupportedSpec, got %v", err)
		}
	})

	t.Run("spec with no paths", func(t *testing.T) {
		_, err := h.service.ValidateImport(ctx, ValidateImportInput{
			ProjectID: project.ID, SourceType: models.ImportSourcePaste,
			Data: []byte(`{"openapi":"3.0.0","info":{"title":"x","version":"1"},"paths":{}}`),
		})
		if !errors.Is(err, openapi.ErrNoPaths) {
			t.Fatalf("expected ErrNoPaths, got %v", err)
		}
	})

	t.Run("oversized document", func(t *testing.T) {
		_, err := h.service.ValidateImport(ctx, ValidateImportInput{
			ProjectID: project.ID, SourceType: models.ImportSourcePaste,
			Data: make([]byte, openapi.MaxDocumentBytes+1),
		})
		if !errors.Is(err, ErrSpecTooLarge) {
			t.Fatalf("expected ErrSpecTooLarge, got %v", err)
		}
	})

	t.Run("unknown source type", func(t *testing.T) {
		_, err := h.service.ValidateImport(ctx, ValidateImportInput{
			ProjectID: project.ID, SourceType: "carrier-pigeon", Data: []byte(petstoreSpec),
		})
		if !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("expected ErrInvalidInput, got %v", err)
		}
	})
}

func TestValidateImportFlagsDuplicatesWithinTheSpec(t *testing.T) {
	h := newTestHarness()
	project := seedProject(t, h)

	// Two paths that normalize to the same route ("/pets" and "/pets/").
	spec := `{
      "openapi":"3.0.0","info":{"title":"Dup","version":"1"},
      "servers":[{"url":"https://a.example"}],
      "paths":{
        "/pets":{"get":{"operationId":"a"}},
        "/pets/":{"get":{"operationId":"b"}}
      }}`
	job := validate(t, h, project, spec, "")

	duplicates := 0
	for _, item := range job.Items {
		if item.Conflict == models.ImportConflictDuplicate {
			duplicates++
			if item.Action != models.ImportActionSkip {
				t.Fatalf("a duplicate must default to skip, got %q", item.Action)
			}
			if item.Selected {
				t.Fatal("a duplicate must not be pre-selected")
			}
		}
	}
	if duplicates != 1 {
		t.Fatalf("expected exactly 1 duplicate to be flagged, got %d", duplicates)
	}
}

func TestCommitImportCreatesSelectedRoutes(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	job := validate(t, h, project, petstoreSpec, "")
	// Deselect one operation to prove selection is honoured.
	selections := map[string]models.ImportCommitSelection{
		"DELETE /pets/{petId}": {Key: "DELETE /pets/{petId}", Selected: false},
	}
	committed, err := h.service.CommitImport(ctx, project, job.ID, selections)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if committed.Status != models.ImportStatusCommitted {
		t.Fatalf("expected a committed job, got %q", committed.Status)
	}
	if committed.CreatedRoutes != 3 || committed.SkippedRoutes != 1 || committed.UpdatedRoutes != 0 || committed.RemovedRoutes != 0 {
		t.Fatalf("unexpected result report: created=%d updated=%d skipped=%d removed=%d",
			committed.CreatedRoutes, committed.UpdatedRoutes, committed.SkippedRoutes, committed.RemovedRoutes)
	}

	routes, total, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 imported routes, got %d", total)
	}
	for _, r := range routes {
		if r.Source != "import" {
			t.Fatalf("imported routes must be marked as such, got %q", r.Source)
		}
		if !r.Enabled {
			t.Fatal("imported routes should be enabled by default")
		}
		if r.MonitorIntervalSecs != project.DefaultIntervalSeconds || r.TimeoutMS != project.DefaultTimeoutMS {
			t.Fatalf("imported routes must inherit project defaults: %+v", r)
		}
		if r.Method == "DELETE" {
			t.Fatal("a deselected operation must not be created")
		}
	}
}

func TestCommitImportRejectsWrongProjectAndDoubleCommit(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	other, err := h.service.CreateProject(ctx, 2, CreateProjectInput{Name: "Other"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	job := validate(t, h, project, petstoreSpec, "")

	// A job belonging to another project must be invisible, not just refused.
	if _, err = h.service.CommitImport(ctx, other, job.ID, nil); !errors.Is(err, domain.ErrImportJobNotFound) {
		t.Fatalf("expected ErrImportJobNotFound for a cross-project commit, got %v", err)
	}
	if _, err = h.service.CommitImport(ctx, project, 999999, nil); !errors.Is(err, domain.ErrImportJobNotFound) {
		t.Fatalf("expected ErrImportJobNotFound for an unknown job, got %v", err)
	}

	commitAll(t, h, project, job)
	if _, err = h.service.CommitImport(ctx, project, job.ID, nil); !errors.Is(err, domain.ErrImportJobCommitted) {
		t.Fatalf("expected ErrImportJobCommitted on replay, got %v", err)
	}
}

func TestGetImportJobIsProjectScoped(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	other, err := h.service.CreateProject(ctx, 2, CreateProjectInput{Name: "Other"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	job := validate(t, h, project, petstoreSpec, "")

	got, err := h.service.GetImportJob(ctx, project.ID, job.ID)
	if err != nil || got == nil {
		t.Fatalf("expected the owning project to read its job, got %+v (%v)", got, err)
	}
	leaked, err := h.service.GetImportJob(ctx, other.ID, job.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if leaked != nil {
		t.Fatal("an import job must not be readable from another project")
	}
}

// --- Re-import behaviour: the core requirement that a second import adds new
// routes, refreshes spec metadata, handles removals, and NEVER silently
// overwrites user-defined monitoring settings. ---

func TestReimportAddsUpdatesAndDetectsRemovals(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	commitAll(t, h, project, validate(t, h, project, petstoreSpec, ""))

	// The user then customises monitoring config on an imported route.
	before, err := h.service.GetRoute(ctx, mustRouteID(t, h, project, "GET", "/pets"))
	if err != nil || before == nil {
		t.Fatalf("load route: %v", err)
	}
	customised, err := h.service.UpdateRoute(ctx, *before, RouteInput{
		MonitorIntervalSecs: 30, TimeoutMS: 1500, Retries: 4,
		ExpectedStatusRange: "200-201", FailureThreshold: 7, RecoverySuccesses: 5,
		Headers: `{"X-Trace":"abc"}`,
	})
	if err != nil {
		t.Fatalf("customise route: %v", err)
	}

	// v2 of the spec: /pets summary changed, /pets/{petId} DELETE removed,
	// /orders added.
	specV2 := `{
      "openapi":"3.0.0","info":{"title":"Petstore","version":"2.0"},
      "servers":[{"url":"https://petstore.example.com/v1"}],
      "paths":{
        "/pets":{
          "get":{"operationId":"listPets","summary":"List all pets (v2)","tags":["pets","v2"]},
          "post":{"operationId":"createPet","summary":"Create a pet","tags":["pets"]}
        },
        "/pets/{petId}":{
          "get":{"operationId":"getPet","summary":"Fetch a pet","tags":["pets"],
                 "parameters":[{"name":"petId","in":"path","required":true,"schema":{"type":"integer"}}]}
        },
        "/orders":{"get":{"operationId":"listOrders","summary":"List orders","tags":["orders"]}}
      }}`

	job := validate(t, h, project, specV2, "")
	items := itemsByKey(job)

	if got := items["GET /pets"]; got.Action != models.ImportActionUpdate || got.Conflict != models.ImportConflictChanged {
		t.Fatalf("a changed operation must be marked changed/update, got %q/%q", got.Action, got.Conflict)
	}
	if got := items["POST /pets"]; got.Action != models.ImportActionSkip || got.Selected {
		t.Fatalf("an unchanged operation must default to skip and be unselected, got %q/%v", got.Action, got.Selected)
	}
	if got := items["GET /orders"]; got.Action != models.ImportActionCreate || !got.Selected {
		t.Fatalf("a new operation must default to create and be selected, got %q/%v", got.Action, got.Selected)
	}
	removed := items["DELETE /pets/{petId}"]
	if removed.Action != models.ImportActionRemove || removed.Conflict != models.ImportConflictRemoved {
		t.Fatalf("a route missing from the spec must be marked removed, got %q/%q", removed.Action, removed.Conflict)
	}
	if removed.Selected {
		t.Fatal("removals must NOT be pre-selected: destructive changes require an explicit opt-in")
	}

	committed := commitAll(t, h, project, job)
	if committed.CreatedRoutes != 1 || committed.UpdatedRoutes != 1 || committed.RemovedRoutes != 0 {
		t.Fatalf("unexpected report: created=%d updated=%d removed=%d skipped=%d",
			committed.CreatedRoutes, committed.UpdatedRoutes, committed.RemovedRoutes, committed.SkippedRoutes)
	}

	// The updated route must have fresh spec metadata...
	after, err := h.service.GetRoute(ctx, customised.ID)
	if err != nil || after == nil {
		t.Fatalf("reload route: %v", err)
	}
	if after.Summary != "List all pets (v2)" {
		t.Fatalf("spec metadata was not refreshed, got %q", after.Summary)
	}
	if !testsupport.ContainsTag(after.Tags, "v2") {
		t.Fatalf("tags were not refreshed, got %v", after.Tags)
	}
	// ...and untouched user-owned monitoring configuration.
	if after.MonitorIntervalSecs != 30 || after.TimeoutMS != 1500 || after.Retries != 4 {
		t.Fatalf("re-import overwrote user monitoring settings: %+v", after)
	}
	if after.ExpectedStatusRange != "200-201" || after.FailureThreshold != 7 || after.RecoverySuccesses != 5 {
		t.Fatalf("re-import overwrote user thresholds: %+v", after)
	}
	if after.Headers != `{"X-Trace":"abc"}` {
		t.Fatalf("re-import overwrote user headers: %q", after.Headers)
	}

	// The removed route still exists and is still enabled, because the user
	// did not select it.
	stale, err := h.service.GetRoute(ctx, mustRouteID(t, h, project, "DELETE", "/pets/{petId}"))
	if err != nil || stale == nil {
		t.Fatalf("a route removed from the spec must not be deleted: %v", err)
	}
	if !stale.Enabled {
		t.Fatal("an unselected removal must not disable the route")
	}
}

func TestReimportDisablesRemovedRoutesWhenExplicitlySelected(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	commitAll(t, h, project, validate(t, h, project, petstoreSpec, ""))

	specV2 := `{
      "openapi":"3.0.0","info":{"title":"Petstore","version":"2.0"},
      "servers":[{"url":"https://petstore.example.com/v1"}],
      "paths":{"/pets":{"get":{"operationId":"listPets","summary":"List pets","tags":["pets"]}}}}`

	job := validate(t, h, project, specV2, "")
	selections := map[string]models.ImportCommitSelection{}
	for _, item := range job.Items {
		if item.Action == models.ImportActionRemove {
			selections[item.Key] = models.ImportCommitSelection{Key: item.Key, Selected: true}
		}
	}
	if len(selections) != 3 {
		t.Fatalf("expected 3 routes to be reported as removed, got %d", len(selections))
	}

	committed, err := h.service.CommitImport(ctx, project, job.ID, selections)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if committed.RemovedRoutes != 3 {
		t.Fatalf("expected 3 removals applied, got %d", committed.RemovedRoutes)
	}

	// Disabled, never hard-deleted: their monitoring history stays intact.
	for _, key := range [][2]string{{"POST", "/pets"}, {"GET", "/pets/{petId}"}, {"DELETE", "/pets/{petId}"}} {
		route, getErr := h.service.GetRoute(ctx, mustRouteID(t, h, project, key[0], key[1]))
		if getErr != nil || route == nil {
			t.Fatalf("%s %s must still exist after removal: %v", key[0], key[1], getErr)
		}
		if route.Enabled {
			t.Fatalf("%s %s should have been disabled", key[0], key[1])
		}
		if route.Status != domain.RouteStatusDisabled {
			t.Fatalf("%s %s should report the disabled state, got %q", key[0], key[1], route.Status)
		}
	}
	// The surviving route is untouched.
	survivor, err := h.service.GetRoute(ctx, mustRouteID(t, h, project, "GET", "/pets"))
	if err != nil || survivor == nil || !survivor.Enabled {
		t.Fatalf("the route still in the spec must stay enabled: %+v (%v)", survivor, err)
	}
}

func TestCommitImportReportsPerRowFailuresWithoutAbortingTheBatch(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)
	commitAll(t, h, project, validate(t, h, project, petstoreSpec, ""))

	// Change two operations so both are "update" candidates.
	specV2 := strings.ReplaceAll(petstoreSpec, "List pets", "List pets v2")
	specV2 = strings.ReplaceAll(specV2, "Fetch a pet", "Fetch a pet v2")
	job := validate(t, h, project, specV2, "")

	// Make one of the two updates fail at the storage layer.
	brokenID := mustRouteID(t, h, project, "GET", "/pets")
	h.routes.UpdateMetadataFailFor[brokenID] = errors.New("deadlock detected")

	committed, err := h.service.CommitImport(ctx, project, job.ID, nil)
	if err != nil {
		t.Fatalf("a single failing row must not abort the commit: %v", err)
	}
	if committed.UpdatedRoutes != 1 {
		t.Fatalf("expected 1 successful update, got %d", committed.UpdatedRoutes)
	}

	var warned int
	for _, item := range committed.Items {
		if item.ValidationWarning != "" {
			warned++
			if !strings.Contains(item.ValidationWarning, "deadlock detected") {
				t.Fatalf("the row failure must be reported verbatim, got %q", item.ValidationWarning)
			}
		}
	}
	if warned != 1 {
		t.Fatalf("expected exactly 1 warned row, got %d", warned)
	}

	// The other route did get its update.
	other, err := h.service.GetRoute(ctx, mustRouteID(t, h, project, "GET", "/pets/{petId}"))
	if err != nil || other == nil {
		t.Fatalf("load: %v", err)
	}
	if other.Summary != "Fetch a pet v2" {
		t.Fatalf("expected the healthy row to still be updated, got %q", other.Summary)
	}
}

func TestImportSupportsSwagger2(t *testing.T) {
	h := newTestHarness()
	project := seedProject(t, h)
	spec := `{
      "swagger":"2.0","info":{"title":"Legacy","version":"1"},
      "host":"legacy.example.com","basePath":"/api/v2","schemes":["https"],
      "paths":{
        "/users":{
          "get":{"operationId":"listUsers","summary":"List users","tags":["users"]},
          "post":{"operationId":"createUser","parameters":[
            {"name":"body","in":"body","required":true,"schema":{"type":"object"}}]}
        }
      }}`
	job := validate(t, h, project, spec, "")
	if job.SpecFormat != openapi.FormatSwagger2 {
		t.Fatalf("expected swagger2, got %q", job.SpecFormat)
	}
	if len(job.Items) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(job.Items))
	}
	for _, item := range job.Items {
		if item.BaseURL != "https://legacy.example.com/api/v2" {
			t.Fatalf("host+basePath+scheme must become the base URL, got %q", item.BaseURL)
		}
	}
	committed := commitAll(t, h, project, job)
	if committed.CreatedRoutes != 2 {
		t.Fatalf("expected 2 created routes, got %d", committed.CreatedRoutes)
	}
}

// --- Acceptance criterion 1: a realistic spec with 500+ routes imports
// completely and the result is searchable and filterable. ---

func buildLargeSpec(resources int) string {
	var b strings.Builder
	b.WriteString(`{"openapi":"3.0.0","info":{"title":"Large API","version":"1.0"},`)
	b.WriteString(`"servers":[{"url":"https://large.example.com/v1"}],"paths":{`)
	for i := 0; i < resources; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		name := fmt.Sprintf("resource%03d", i)
		tag := fmt.Sprintf("group%d", i%10)
		fmt.Fprintf(&b, `"/%s":{`, name)
		fmt.Fprintf(&b, `"get":{"operationId":"list_%s","summary":"List %s","tags":["%s"]},`, name, name, tag)
		fmt.Fprintf(&b, `"post":{"operationId":"create_%s","summary":"Create %s","tags":["%s"]}`, name, name, tag)
		b.WriteString(`},`)
		fmt.Fprintf(&b, `"/%s/{id}":{`, name)
		fmt.Fprintf(&b, `"get":{"operationId":"get_%s","summary":"Get %s","tags":["%s"],`, name, name, tag)
		b.WriteString(`"parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"integer"}}]},`)
		fmt.Fprintf(&b, `"delete":{"operationId":"delete_%s","summary":"Delete %s","tags":["%s"],"deprecated":%t}`, name, name, tag, i%5 == 0)
		b.WriteString(`}`)
	}
	b.WriteString(`}}`)
	return b.String()
}

func TestImportFiveHundredPlusRoutesEndToEnd(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project := seedProject(t, h)

	const resources = 150 // 150 resources x 4 operations = 600 routes
	const expected = resources * 4

	job := validate(t, h, project, buildLargeSpec(resources), "")
	if job.TotalParsed != expected {
		t.Fatalf("expected %d parsed operations, got %d", expected, job.TotalParsed)
	}
	if len(job.Items) != expected {
		t.Fatalf("expected %d preview items, got %d", expected, len(job.Items))
	}
	for _, item := range job.Items {
		if item.Action != models.ImportActionCreate || !item.Selected {
			t.Fatalf("every item in a first import must be a pre-selected create, got %q/%v for %s", item.Action, item.Selected, item.Key)
		}
	}

	committed := commitAll(t, h, project, job)
	if committed.CreatedRoutes != expected {
		t.Fatalf("expected %d routes created, got %d", expected, committed.CreatedRoutes)
	}

	_, total, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != expected {
		t.Fatalf("expected %d routes stored, got %d", expected, total)
	}

	// Searchable.
	found, hits, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Search: "resource042"})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if hits != 4 || len(found) != 4 {
		t.Fatalf("expected the 4 operations of resource042, got %d", hits)
	}

	// Filterable by method, tag and deprecation.
	_, gets, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Method: "GET"})
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if gets != resources*2 {
		t.Fatalf("expected %d GET routes, got %d", resources*2, gets)
	}
	_, tagged, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Tag: "group3"})
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if tagged != (resources/10)*4 {
		t.Fatalf("expected %d routes in group3, got %d", (resources/10)*4, tagged)
	}
	deprecated := true
	_, deprecatedCount, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Deprecated: &deprecated})
	if err != nil {
		t.Fatalf("filter: %v", err)
	}
	if deprecatedCount != resources/5 {
		t.Fatalf("expected %d deprecated routes, got %d", resources/5, deprecatedCount)
	}

	// Paginated.
	page, _, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID, Limit: 50, Offset: 100, SortBy: "path"})
	if err != nil {
		t.Fatalf("paginate: %v", err)
	}
	if len(page) != 50 {
		t.Fatalf("expected a page of 50, got %d", len(page))
	}

	// A second identical import is a complete no-op.
	rerun := validate(t, h, project, buildLargeSpec(resources), "")
	for _, item := range rerun.Items {
		if item.Action != models.ImportActionSkip || item.Selected {
			t.Fatalf("an unchanged re-import must skip everything, got %q/%v for %s", item.Action, item.Selected, item.Key)
		}
	}
	recommitted := commitAll(t, h, project, rerun)
	if recommitted.CreatedRoutes != 0 || recommitted.UpdatedRoutes != 0 || recommitted.RemovedRoutes != 0 {
		t.Fatalf("an unchanged re-import must change nothing: %+v", recommitted)
	}
	if recommitted.SkippedRoutes != expected {
		t.Fatalf("expected all %d items to be skipped, got %d", expected, recommitted.SkippedRoutes)
	}
	_, afterTotal, err := h.service.ListRoutes(ctx, models.RouteFilter{ProjectID: project.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if afterTotal != expected {
		t.Fatalf("route count changed after a no-op re-import: %d", afterTotal)
	}
}

func mustRouteID(t *testing.T, h *testHarness, project models.Project, method, path string) int64 {
	t.Helper()
	route, err := h.routes.GetRouteByMethodPath(context.Background(), project.ID, method, path)
	if err != nil || route == nil {
		t.Fatalf("route %s %s not found: %v", method, path, err)
	}
	return route.ID
}
