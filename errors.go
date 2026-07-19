package warcraftlogs

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Khan/genqlient/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// ErrNotFound is returned by single-entity lookups when the API resolves the
// query but no matching entity exists.
var ErrNotFound = errors.New("warcraftlogs: not found")

// ErrBlocked reports that a response was served by the CDN in front of the API
// rather than the API itself. Match it with [IsBlocked].
var ErrBlocked = errors.New("warcraftlogs: blocked by CDN")

// CDNError is returned when the CDN answers with an HTML challenge or error
// page instead of a GraphQL response. It unwraps to [ErrBlocked].
type CDNError struct {
	StatusCode int
	URL        string
	// Title is the <title> of the returned page, e.g. "Just a moment...". It is
	// empty if the page had none.
	Title string
}

func (e *CDNError) Error() string {
	if e.Title != "" {
		return fmt.Sprintf("warcraftlogs: blocked by CDN (HTTP %d, %q)", e.StatusCode, e.Title)
	}
	return fmt.Sprintf("warcraftlogs: blocked by CDN (HTTP %d)", e.StatusCode)
}

func (e *CDNError) Unwrap() error { return ErrBlocked }

// orNotFound returns v, or ErrNotFound if v is nil.
func orNotFound[T any](v *T) (*T, error) {
	if v == nil {
		return nil, ErrNotFound
	}
	return v, nil
}

// Location is a position in the request document that an error refers to.
type Location struct {
	Line   int
	Column int
}

// GraphQLError is a single error entry from a GraphQL response.
type GraphQLError struct {
	Message string
	// Path is the dotted response path the error applies to, such as
	// "reportData.report", with list elements as "[0]". It is empty when the
	// error is not tied to a field.
	Path       string
	Locations  []Location
	Extensions map[string]any
}

func (e GraphQLError) Error() string { return e.Message }

// GraphQLErrors returns the GraphQL-level errors carried by err, or nil if err
// is not one. A response may contain both partial data and errors.
func GraphQLErrors(err error) []GraphQLError {
	var list gqlerror.List
	if !errors.As(err, &list) {
		return nil
	}
	out := make([]GraphQLError, 0, len(list))
	for _, e := range list {
		if e == nil {
			continue
		}
		ge := GraphQLError{
			Message:    e.Message,
			Path:       e.Path.String(),
			Extensions: e.Extensions,
		}
		if len(e.Locations) > 0 {
			ge.Locations = make([]Location, len(e.Locations))
			for i, l := range e.Locations {
				ge.Locations[i] = Location{Line: l.Line, Column: l.Column}
			}
		}
		out = append(out, ge)
	}
	return out
}

// HTTPStatus returns the status code if err was caused by a non-2xx response,
// whether it came from the API or from the CDN in front of it.
func HTTPStatus(err error) (int, bool) {
	var he *graphql.HTTPError
	if errors.As(err, &he) {
		return he.StatusCode, true
	}
	var ce *CDNError
	if errors.As(err, &ce) {
		return ce.StatusCode, true
	}
	return 0, false
}

// ErrorCode returns the "code" or "category" extension of the first GraphQL
// error carrying one, or "" if there is none.
func ErrorCode(err error) string {
	for _, ge := range GraphQLErrors(err) {
		for _, key := range []string{"code", "category"} {
			if s, ok := ge.Extensions[key].(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// IsBlocked reports whether the request was rejected by the CDN in front of the
// API rather than reaching it. See [CDNError].
func IsBlocked(err error) bool { return errors.Is(err, ErrBlocked) }

// IsRateLimited reports whether err was caused by exhausting the hourly point
// budget, reported either as an HTTP 429 or as a GraphQL error.
func IsRateLimited(err error) bool {
	if IsBlocked(err) {
		return false
	}
	if code, ok := HTTPStatus(err); ok && code == http.StatusTooManyRequests {
		return true
	}
	return hasGraphQLMessage(err, "exhausted", "rate limit")
}

// IsUnauthorized reports whether err was caused by missing, expired or
// insufficient credentials, reported either as an HTTP 401 or 403 or as a
// GraphQL error. A CDN challenge is not an auth failure, so [IsBlocked] takes
// precedence.
func IsUnauthorized(err error) bool {
	if IsBlocked(err) {
		return false
	}
	if code, ok := HTTPStatus(err); ok && (code == http.StatusUnauthorized || code == http.StatusForbidden) {
		return true
	}
	return hasGraphQLMessage(err, "do not have permission", "unauthenticated", "unauthorized")
}

// hasGraphQLMessage matches GraphQL error text case-insensitively. The API does
// not classify these errors in extensions, so the message is all there is.
func hasGraphQLMessage(err error, substrings ...string) bool {
	for _, ge := range GraphQLErrors(err) {
		msg := strings.ToLower(ge.Message)
		for _, s := range substrings {
			if strings.Contains(msg, s) {
				return true
			}
		}
	}
	return false
}
