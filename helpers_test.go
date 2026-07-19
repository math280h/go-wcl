package warcraftlogs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Khan/genqlient/graphql"
)

// stubGQL is a GraphQL server that answers with canned bodies in order and
// records the variables of every request it receives.
type stubGQL struct {
	bodies []string
	vars   []map[string]any
}

// newStubGQL returns a client pointed at a server replying with bodies, one per
// request. Asking for more requests than there are bodies fails the test.
func newStubGQL(t *testing.T, bodies ...string) (*Client, *stubGQL) {
	t.Helper()
	s := &stubGQL{bodies: bodies}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		s.vars = append(s.vars, req.Variables)

		if len(s.vars) > len(s.bodies) {
			t.Errorf("unexpected request %d, only %d responses canned", len(s.vars), len(s.bodies))
			http.Error(w, "no more responses", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(s.bodies[len(s.vars)-1]))
	}))
	t.Cleanup(srv.Close)
	return &Client{gql: graphql.NewClient(srv.URL, srv.Client()), endpoint: srv.URL}, s
}

func (s *stubGQL) calls() int { return len(s.vars) }

// last returns the variables of the most recent request.
func (s *stubGQL) last() map[string]any {
	if len(s.vars) == 0 {
		return nil
	}
	return s.vars[len(s.vars)-1]
}

// sent reports whether a variable was included in the most recent request.
func (s *stubGQL) sent(name string) bool {
	_, ok := s.last()[name]
	return ok
}

// variable returns the named variable from each request in order, using zero
// for requests that omitted it.
func (s *stubGQL) variable(name string) []float64 {
	out := make([]float64, len(s.vars))
	for i, v := range s.vars {
		if f, ok := v[name].(float64); ok {
			out[i] = f
		}
	}
	return out
}
