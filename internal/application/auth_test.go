package application

import (
	"context"
	"errors"
	"testing"

	"argus/internal/models"
)

func TestRegisterCreatesUserAndIssuesToken(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()

	result, err := h.service.Register(ctx, "  Alice@Example.COM ", "correct horse battery", "Alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.Email != "alice@example.com" {
		t.Fatalf("expected the email to be normalized, got %q", result.User.Email)
	}
	if result.Token == "" {
		t.Fatal("expected a bearer token to be issued")
	}
	stored, _ := h.users.GetUserByEmail(ctx, "alice@example.com")
	if stored == nil {
		t.Fatal("expected the user to be persisted")
	}
	if stored.PasswordHash == "correct horse battery" || stored.PasswordHash == "" {
		t.Fatal("password must be stored as a hash, never in cleartext")
	}
	if result.User.PasswordHash != "" {
		t.Fatal("the password hash must never be returned to callers")
	}
}

func TestRegisterRejectsBadInput(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()

	cases := []struct {
		name     string
		email    string
		password string
		want     error
	}{
		{"missing at sign", "not-an-email", "longenoughpassword", ErrInvalidEmail},
		{"missing domain dot", "user@localhost", "longenoughpassword", ErrInvalidEmail},
		{"empty email", "", "longenoughpassword", ErrInvalidEmail},
		{"short password", "user@example.com", "short", ErrWeakPassword},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := h.service.Register(ctx, tc.email, tc.password, ""); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestRegisterRejectsDuplicateEmail(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	if _, err := h.service.Register(ctx, "dup@example.com", "longenoughpassword", ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Case-insensitively the same account.
	if _, err := h.service.Register(ctx, "DUP@example.com", "longenoughpassword", ""); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestLogin(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	if _, err := h.service.Register(ctx, "bob@example.com", "longenoughpassword", "Bob"); err != nil {
		t.Fatalf("register: %v", err)
	}

	t.Run("valid credentials", func(t *testing.T) {
		result, err := h.service.Login(ctx, "BOB@example.com", "longenoughpassword")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Token == "" {
			t.Fatal("expected a token")
		}
		if result.User.PasswordHash != "" {
			t.Fatal("password hash leaked in login response")
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		if _, err := h.service.Login(ctx, "bob@example.com", "wrongpassword"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("unknown account returns the same error", func(t *testing.T) {
		// Identical error for unknown user and wrong password so the API
		// cannot be used to enumerate registered addresses.
		if _, err := h.service.Login(ctx, "nobody@example.com", "longenoughpassword"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestAuthenticate(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	reg, err := h.service.Register(ctx, "carol@example.com", "longenoughpassword", "Carol")
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	t.Run("valid token", func(t *testing.T) {
		user, authErr := h.service.Authenticate(ctx, reg.Token)
		if authErr != nil {
			t.Fatalf("unexpected error: %v", authErr)
		}
		if user == nil || user.ID != reg.User.ID {
			t.Fatalf("expected user %d, got %+v", reg.User.ID, user)
		}
		if user.PasswordHash != "" {
			t.Fatal("password hash leaked from Authenticate")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		if _, authErr := h.service.Authenticate(ctx, "  "); !errors.Is(authErr, ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got %v", authErr)
		}
	})

	t.Run("unknown token", func(t *testing.T) {
		if _, authErr := h.service.Authenticate(ctx, "deadbeef"); !errors.Is(authErr, ErrInvalidToken) {
			t.Fatalf("expected ErrInvalidToken, got %v", authErr)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		h.tokens.ExpireAll()
		if _, authErr := h.service.Authenticate(ctx, reg.Token); !errors.Is(authErr, ErrInvalidToken) {
			t.Fatalf("expected an expired token to be rejected, got %v", authErr)
		}
	})
}

func TestLogoutInvalidatesToken(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	reg, err := h.service.Register(ctx, "dave@example.com", "longenoughpassword", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if err = h.service.Logout(ctx, reg.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err = h.service.Authenticate(ctx, reg.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected the token to be revoked, got %v", err)
	}
}

// TestRegisterDefaultsNameToEmail documents the fallback so the UI always has
// something to show.
func TestRegisterDefaultsNameToEmail(t *testing.T) {
	h := newTestHarness()
	result, err := h.service.Register(context.Background(), "eve@example.com", "longenoughpassword", "   ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.User.Name != "eve@example.com" {
		t.Fatalf("expected the name to default to the email, got %q", result.User.Name)
	}
}

func TestTokensAreUniquePerSession(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	reg, err := h.service.Register(ctx, "frank@example.com", "longenoughpassword", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	login, err := h.service.Login(ctx, "frank@example.com", "longenoughpassword")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if reg.Token == login.Token {
		t.Fatal("each session must get its own token")
	}
	// Revoking one session must not revoke the other.
	if err = h.service.Logout(ctx, login.Token); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err = h.service.Authenticate(ctx, reg.Token); err != nil {
		t.Fatalf("the original session should still be valid, got %v", err)
	}
}

func TestRegisterMakesOwnerOfCreatedProjects(t *testing.T) {
	h := newTestHarness()
	ctx := context.Background()
	reg, err := h.service.Register(ctx, "grace@example.com", "longenoughpassword", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	project, err := h.service.CreateProject(ctx, reg.User.ID, CreateProjectInput{Name: "Payments API"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	member, err := h.projects.GetProjectMember(ctx, project.ID, reg.User.ID)
	if err != nil || member == nil {
		t.Fatalf("expected an owner membership row, got %+v (%v)", member, err)
	}
	if member.Role != models.ProjectRoleOwner {
		t.Fatalf("expected owner role, got %q", member.Role)
	}
}
