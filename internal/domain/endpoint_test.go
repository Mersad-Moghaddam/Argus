package domain

import (
	"errors"
	"testing"
)

func TestNormalizeEndpointCanonicalizesStructuredInputs(t *testing.T) {
	normalized, err := NormalizeEndpoint(" get ", " HTTPS://BÜCHER.example.:443/api/../v1/ ", "pets/%7Earchive/{petId}")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got, want := normalized.Method, "GET"; got != want {
		t.Fatalf("method = %q, want %q", got, want)
	}
	if got, want := normalized.BaseURL, "https://xn--bcher-kva.example/v1"; got != want {
		t.Fatalf("base URL = %q, want %q", got, want)
	}
	if got, want := normalized.RouteTemplate, "/pets/~archive/{petId}"; got != want {
		t.Fatalf("route = %q, want %q", got, want)
	}
	if got, want := normalized.CanonicalIdentity, "GET https://xn--bcher-kva.example/v1/pets/~archive/{petId}"; got != want {
		t.Fatalf("identity = %q, want %q", got, want)
	}
	if normalized.FetchTarget != "" {
		t.Fatalf("template must not become a fetch target: %q", normalized.FetchTarget)
	}
}

func TestNormalizeEndpointBuildsConcreteFetchTargetWithURLResolution(t *testing.T) {
	normalized, err := NormalizeEndpoint("HEAD", "https://example.com/api", "/health")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got, want := normalized.FetchTarget, "https://example.com/api/health"; got != want {
		t.Fatalf("fetch target = %q, want %q", got, want)
	}
}

func TestNormalizeEndpointRejectsAmbiguousAndUnsafeInputs(t *testing.T) {
	cases := []struct{ name, base, path, code string }{
		{"missing scheme", "example.com", "/x", "absolute_url_required"},
		{"userinfo", "https://user:pass@example.com", "/x", "userinfo_not_allowed"},
		{"fragment", "https://example.com/#fragment", "/x", "fragment_not_allowed"},
		{"query", "https://example.com/?a=b", "/x", "query_not_allowed"},
		{"encoded slash", "https://example.com", "/a%2fb", "encoded_separator_not_allowed"},
		{"literal backslash", "https://example.com", `/a\b`, "unsafe_character"},
		{"bad template", "https://example.com", "/pets/{9id}", "invalid_template_parameter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeEndpoint("GET", tc.base, tc.path)
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, ErrInvalidRoute) {
				t.Fatalf("must preserve ErrInvalidRoute, got %v", err)
			}
			if got := ValidationCode(err); got != tc.code {
				t.Fatalf("code = %q, want %q", got, tc.code)
			}
		})
	}
}

func TestNormalizeRouteTemplatePreservesRepeatedAndUnifiesTrailingSlashes(t *testing.T) {
	path, err := NormalizeRouteTemplate("/v1//pets/")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got, want := path, "/v1//pets"; got != want {
		t.Fatalf("path = %q, want %q", got, want)
	}
}

func TestValidationCodeAndMessageForRoutePolicyErrors(t *testing.T) {
	tests := []struct {
		err     error
		code    string
		message string
	}{
		{ErrDuplicateRoute, "duplicate_route", ErrDuplicateRoute.Error()},
		{ErrUnsafeSynthetic, "unsafe_synthetic", ErrUnsafeSynthetic.Error()},
		{ErrInvalidInput, "invalid_input", ErrInvalidInput.Error()},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			if got := ValidationCode(tt.err); got != tt.code {
				t.Fatalf("ValidationCode() = %q, want %q", got, tt.code)
			}
			if got := ValidationMessage(tt.err); got != tt.message {
				t.Fatalf("ValidationMessage() = %q, want %q", got, tt.message)
			}
		})
	}
}
