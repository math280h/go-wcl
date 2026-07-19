package warcraftlogs

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/Khan/genqlient/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func TestGraphQLErrors(t *testing.T) {
	list := gqlerror.List{
		nil,
		{
			Message:    "This report does not exist.",
			Path:       ast.Path{ast.PathName("reportData"), ast.PathName("report")},
			Locations:  []gqlerror.Location{{Line: 4, Column: 3}},
			Extensions: map[string]any{"category": "graphql"},
		},
		{Message: "no path"},
		{Message: "indexed", Path: ast.Path{ast.PathName("fights"), ast.PathIndex(2)}},
	}

	got := GraphQLErrors(fmt.Errorf("wrapped: %w", list))
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (nil entry dropped)", len(got))
	}
	if got[0].Path != "reportData.report" {
		t.Errorf("Path = %q", got[0].Path)
	}
	if got[0].Error() != "This report does not exist." {
		t.Errorf("Error() = %q", got[0].Error())
	}
	if len(got[0].Locations) != 1 || got[0].Locations[0].Line != 4 || got[0].Locations[0].Column != 3 {
		t.Errorf("Locations = %+v", got[0].Locations)
	}
	if got[0].Extensions["category"] != "graphql" {
		t.Errorf("Extensions = %v", got[0].Extensions)
	}
	if got[1].Path != "" {
		t.Errorf("empty path = %q, want \"\"", got[1].Path)
	}
	if got[2].Path != "fights[2]" {
		t.Errorf("indexed path = %q", got[2].Path)
	}
}

func TestGraphQLErrorsIgnoresOtherErrors(t *testing.T) {
	if got := GraphQLErrors(errors.New("plain")); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := GraphQLErrors(nil); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestHTTPStatusAndRateLimited(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &graphql.HTTPError{StatusCode: http.StatusTooManyRequests})
	status, ok := HTTPStatus(err)
	if !ok || status != http.StatusTooManyRequests {
		t.Fatalf("HTTPStatus = %d, %v", status, ok)
	}
	if !IsRateLimited(err) {
		t.Error("IsRateLimited = false, want true")
	}

	other := fmt.Errorf("wrapped: %w", &graphql.HTTPError{StatusCode: http.StatusInternalServerError})
	if IsRateLimited(other) {
		t.Error("IsRateLimited(500) = true, want false")
	}
	if _, ok := HTTPStatus(errors.New("plain")); ok {
		t.Error("HTTPStatus(plain) ok = true, want false")
	}
}

func TestIsUnauthorized(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		if !IsUnauthorized(&graphql.HTTPError{StatusCode: code}) {
			t.Errorf("IsUnauthorized(%d) = false, want true", code)
		}
	}
	if IsUnauthorized(&graphql.HTTPError{StatusCode: http.StatusNotFound}) {
		t.Error("IsUnauthorized(404) = true, want false")
	}
	// The API reports an unreadable report as nonexistent, not as a permission
	// failure, so no GraphQL error classifies as unauthorized.
	list := gqlerror.List{{Message: "This report does not exist."}}
	if IsUnauthorized(fmt.Errorf("wrapped: %w", list)) {
		t.Error("IsUnauthorized(graphql) = true, want false")
	}
}

func TestCDNError(t *testing.T) {
	err := error(&CDNError{StatusCode: http.StatusForbidden, URL: "https://example.com", Title: "Just a moment..."})
	if !IsBlocked(fmt.Errorf("wrapped: %w", err)) {
		t.Error("IsBlocked = false, want true")
	}
	if !errors.Is(err, ErrBlocked) {
		t.Error("errors.Is(err, ErrBlocked) = false, want true")
	}
	if got := err.Error(); got != `warcraftlogs: blocked by CDN (HTTP 403, "Just a moment...")` {
		t.Errorf("Error() = %q", got)
	}
	untitled := &CDNError{StatusCode: http.StatusServiceUnavailable}
	if got := untitled.Error(); got != "warcraftlogs: blocked by CDN (HTTP 503)" {
		t.Errorf("Error() = %q", got)
	}
	if IsBlocked(errors.New("plain")) {
		t.Error("IsBlocked(plain) = true, want false")
	}

	// A CDN block reports its status but is not an auth or rate-limit failure.
	if status, ok := HTTPStatus(err); !ok || status != http.StatusForbidden {
		t.Errorf("HTTPStatus = %d, %v; want 403, true", status, ok)
	}
	if IsUnauthorized(err) {
		t.Error("IsUnauthorized(403 CDN block) = true, want false")
	}
	if IsRateLimited(&CDNError{StatusCode: http.StatusTooManyRequests}) {
		t.Error("IsRateLimited(429 CDN block) = true, want false")
	}
}

func TestOrNotFound(t *testing.T) {
	v := &Zone{Id: 1}
	got, err := orNotFound(v)
	if err != nil || got != v {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := orNotFound[Zone](nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
