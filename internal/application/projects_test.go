package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"argus/internal/domain"
	"argus/internal/models"
)

func TestCreateProjectAppliesDefaultsAndClamps(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()

	t.Run("defaults", func(t *testing.T) {
		project, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "Billing API"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.Status != domain.ProjectStatusActive {
			t.Fatalf("expected a new project to be active, got %q", project.Status)
		}
		if project.DefaultIntervalSeconds != 300 || project.DefaultTimeoutMS != 5000 || project.DefaultRetries != 1 {
			t.Fatalf("unexpected monitoring defaults: %+v", project)
		}
		if project.FailureThreshold != domain.DefaultFailureThreshold || project.RecoverySuccessThreshold != domain.DefaultRecoverySuccesses {
			t.Fatalf("unexpected incident defaults: %+v", project)
		}
		if !strings.HasPrefix(project.Slug, "billing-api-") {
			t.Fatalf("expected a slugified name with a unique suffix, got %q", project.Slug)
		}
	})

	t.Run("out-of-range values are clamped, not rejected", func(t *testing.T) {
		project, err := h.service.CreateProject(ctx, 1, CreateProjectInput{
			Name: "Extreme", DefaultIntervalSeconds: 99999999, DefaultTimeoutMS: 1,
			DefaultRetries: 99, FailureThreshold: 999, RecoverySuccessThreshold: 999,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if project.DefaultIntervalSeconds != 86400 {
			t.Fatalf("interval not clamped: %d", project.DefaultIntervalSeconds)
		}
		if project.DefaultTimeoutMS != 200 {
			t.Fatalf("timeout not clamped: %d", project.DefaultTimeoutMS)
		}
		if project.DefaultRetries != 5 || project.FailureThreshold != 20 || project.RecoverySuccessThreshold != 20 {
			t.Fatalf("thresholds not clamped: %+v", project)
		}
	})

	t.Run("name is required", func(t *testing.T) {
		if _, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "   "}); !errors.Is(err, ErrProjectNameRequired) {
			t.Fatalf("expected ErrProjectNameRequired, got %v", err)
		}
	})
}

func TestProjectSlugsAreUniquePerName(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	a, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "Same Name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "Same Name"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Slug == b.Slug {
		t.Fatalf("two projects with the same name must not collide on slug: %q", a.Slug)
	}
}

// TestAuthorizeProject is the authorization matrix. The security-critical
// property is that a non-member and a nonexistent project are indistinguishable.
func TestAuthorizeProject(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()

	const ownerID, editorID, viewerID, strangerID = int64(1), int64(2), int64(3), int64(4)
	project, err := h.service.CreateProject(ctx, ownerID, CreateProjectInput{Name: "Secure API"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if err = h.projects.AddProjectMember(ctx, models.ProjectMember{ProjectID: project.ID, UserID: editorID, Role: models.ProjectRoleEditor}); err != nil {
		t.Fatalf("add editor: %v", err)
	}
	if err = h.projects.AddProjectMember(ctx, models.ProjectMember{ProjectID: project.ID, UserID: viewerID, Role: models.ProjectRoleViewer}); err != nil {
		t.Fatalf("add viewer: %v", err)
	}

	cases := []struct {
		name      string
		projectID int64
		userID    int64
		minRole   string
		wantErr   error
	}{
		{"owner can act as owner", project.ID, ownerID, models.ProjectRoleOwner, nil},
		{"owner can act as editor", project.ID, ownerID, models.ProjectRoleEditor, nil},
		{"owner can act as viewer", project.ID, ownerID, models.ProjectRoleViewer, nil},
		{"editor can act as editor", project.ID, editorID, models.ProjectRoleEditor, nil},
		{"editor can act as viewer", project.ID, editorID, models.ProjectRoleViewer, nil},
		{"editor cannot act as owner", project.ID, editorID, models.ProjectRoleOwner, ErrInsufficientRole},
		{"viewer can read", project.ID, viewerID, models.ProjectRoleViewer, nil},
		{"viewer cannot write", project.ID, viewerID, models.ProjectRoleEditor, ErrInsufficientRole},
		{"viewer cannot own", project.ID, viewerID, models.ProjectRoleOwner, ErrInsufficientRole},
		{"non-member sees not-found", project.ID, strangerID, models.ProjectRoleViewer, domain.ErrProjectNotFound},
		{"nonexistent project sees not-found", 999999, ownerID, models.ProjectRoleViewer, domain.ErrProjectNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, role, authErr := h.service.AuthorizeProject(ctx, tc.projectID, tc.userID, tc.minRole)
			if tc.wantErr != nil {
				if !errors.Is(authErr, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, authErr)
				}
				if got != nil {
					t.Fatal("no project must be returned on an authorization failure")
				}
				return
			}
			if authErr != nil {
				t.Fatalf("unexpected error: %v", authErr)
			}
			if got == nil || got.ID != tc.projectID {
				t.Fatalf("expected project %d, got %+v", tc.projectID, got)
			}
			if got.ViewerRole != role || role == "" {
				t.Fatalf("expected the caller's role to be reported, got %q/%q", got.ViewerRole, role)
			}
		})
	}
}

// TestAuthorizeProjectDoesNotLeakExistence asserts the two "denied" paths are
// byte-for-byte the same error, which is what lets the handler layer return
// 404 for both and defeat project-ID enumeration.
func TestAuthorizeProjectDoesNotLeakExistence(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "Private"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	_, _, existsButForbidden := h.service.AuthorizeProject(ctx, project.ID, 42, models.ProjectRoleViewer)
	_, _, doesNotExist := h.service.AuthorizeProject(ctx, project.ID+5000, 42, models.ProjectRoleViewer)
	if existsButForbidden == nil || doesNotExist == nil {
		t.Fatal("both cases must be denied")
	}
	if existsButForbidden.Error() != doesNotExist.Error() {
		t.Fatalf("errors must be indistinguishable: %q vs %q", existsButForbidden, doesNotExist)
	}
}

func TestUpdateArchiveAndDeleteProject(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	project, err := h.service.CreateProject(ctx, 1, CreateProjectInput{Name: "Lifecycle"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	updated, err := h.service.UpdateProject(ctx, project, UpdateProjectInput{Name: "Lifecycle v2", Description: "  edited  ", DefaultIntervalSeconds: 60})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Lifecycle v2" || updated.Description != "edited" || updated.DefaultIntervalSeconds != 60 {
		t.Fatalf("unexpected update result: %+v", updated)
	}
	if updated.Slug != project.Slug {
		t.Fatalf("renaming must not change the slug (it is a stable identifier): %q -> %q", project.Slug, updated.Slug)
	}

	if _, err = h.service.UpdateProject(ctx, project, UpdateProjectInput{Name: "  "}); !errors.Is(err, ErrProjectNameRequired) {
		t.Fatalf("expected a blank name to be rejected, got %v", err)
	}

	if err = h.service.SetProjectStatus(ctx, project.ID, domain.ProjectStatusArchived); err != nil {
		t.Fatalf("archive: %v", err)
	}
	stored, _ := h.service.GetProject(ctx, project.ID)
	if stored == nil || stored.Status != domain.ProjectStatusArchived {
		t.Fatalf("expected the project to be archived, got %+v", stored)
	}
	if err = h.service.SetProjectStatus(ctx, project.ID, domain.ProjectStatusActive); err != nil {
		t.Fatalf("unarchive: %v", err)
	}
	if err = h.service.SetProjectStatus(ctx, project.ID, "bogus"); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("expected an invalid status to be rejected, got %v", err)
	}

	if err = h.service.DeleteProject(ctx, project.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if gone, _ := h.service.GetProject(ctx, project.ID); gone != nil {
		t.Fatal("expected the project to be deleted")
	}
}

func TestListProjectsFiltersByMembershipAndStatus(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	const mine, theirs = int64(1), int64(2)

	active, err := h.service.CreateProject(ctx, mine, CreateProjectInput{Name: "Active One"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	archived, err := h.service.CreateProject(ctx, mine, CreateProjectInput{Name: "Archived One"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err = h.service.SetProjectStatus(ctx, archived.ID, domain.ProjectStatusArchived); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if _, err = h.service.CreateProject(ctx, theirs, CreateProjectInput{Name: "Someone Elses"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	all, total, err := h.service.ListProjects(ctx, mine, models.ProjectFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(all) != 2 {
		t.Fatalf("expected only the caller's 2 projects, got %d (total %d)", len(all), total)
	}

	onlyActive, _, err := h.service.ListProjects(ctx, mine, models.ProjectFilter{Status: domain.ProjectStatusActive})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(onlyActive) != 1 || onlyActive[0].ID != active.ID {
		t.Fatalf("status filter failed: %+v", onlyActive)
	}

	searched, _, err := h.service.ListProjects(ctx, mine, models.ProjectFilter{Search: "archived"})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(searched) != 1 || searched[0].ID != archived.ID {
		t.Fatalf("search filter failed: %+v", searched)
	}

	none, _, err := h.service.ListProjects(ctx, 999, models.ProjectFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("a user with no memberships must see nothing, got %d", len(none))
	}
}
