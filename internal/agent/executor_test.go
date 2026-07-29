package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecuteAssignmentUsesOnlyTheSignedOperation(t *testing.T) {
	var gotMethod string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	ok, summary := ExecuteAssignment(context.Background(), Assignment{ID: 1, Method: http.MethodHead, Target: target.URL, IntervalSecs: 15, TimeoutMS: 200}, nil)
	if !ok || summary != "" || gotMethod != http.MethodHead {
		t.Fatalf("head assignment: ok=%t summary=%q method=%q", ok, summary, gotMethod)
	}
}

func TestExecuteAssignmentDoesNotFollowRedirects(t *testing.T) {
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://should-not-be-contacted.invalid", http.StatusFound)
	}))
	defer redirect.Close()

	ok, summary := ExecuteAssignment(context.Background(), Assignment{ID: 1, Method: http.MethodGet, Target: redirect.URL, IntervalSecs: 15, TimeoutMS: 200}, nil)
	if ok || summary != "unexpected HTTP status 302" {
		t.Fatalf("redirect assignment: ok=%t summary=%q", ok, summary)
	}
}

func TestExecuteAssignmentRejectsUnsignedLikeWork(t *testing.T) {
	for _, assignment := range []Assignment{
		{ID: 0, Method: http.MethodGet, Target: "https://example.com", TimeoutMS: 200},
		{ID: 1, Method: http.MethodPost, Target: "https://example.com", TimeoutMS: 200},
		{ID: 1, Method: http.MethodGet, Target: "https://user:pass@example.com", TimeoutMS: 200},
		{ID: 1, Method: http.MethodGet, Target: "https://example.com#fragment", TimeoutMS: 200},
	} {
		if ok, _ := ExecuteAssignment(context.Background(), assignment, nil); ok {
			t.Fatalf("unsafe assignment was accepted: %+v", assignment)
		}
	}
}
