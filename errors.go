package warcraftlogs

import (
	"errors"
	"net/http"

	"github.com/Khan/genqlient/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// ErrNotFound is returned by single-entity lookups when the API resolves the
// query but no matching entity exists.
var ErrNotFound = errors.New("warcraftlogs: not found")

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

// HTTPStatus returns the status code if err was caused by a non-2xx response.
func HTTPStatus(err error) (int, bool) {
	var he *graphql.HTTPError
	if errors.As(err, &he) {
		return he.StatusCode, true
	}
	return 0, false
}

// IsRateLimited reports whether err was caused by an HTTP 429 response.
func IsRateLimited(err error) bool {
	code, ok := HTTPStatus(err)
	return ok && code == http.StatusTooManyRequests
}
