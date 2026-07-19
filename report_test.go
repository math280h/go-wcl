package warcraftlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Khan/genqlient/graphql"
)

func TestReportActorsSendsFilterAndUnwraps(t *testing.T) {
	var vars map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decoding request: %v", err)
		}
		vars = req.Variables
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"reportData":{"report":{"masterData":{"actors":[
			{"id":1,"name":"Zesyis","type":"Player","subType":"DeathKnight"}]}}}}}`)
	}))
	defer srv.Close()
	client := &Client{gql: graphql.NewClient(srv.URL, srv.Client()), endpoint: srv.URL}

	actors, err := client.ReportActors(context.Background(), ReportActorsParams{
		Code: "abc", Type: ActorPlayer, SubType: "DeathKnight",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(actors) != 1 || actors[0].Name != "Zesyis" {
		t.Fatalf("actors = %+v", actors)
	}
	if vars["actorType"] != "Player" || vars["actorSubType"] != "DeathKnight" {
		t.Errorf("variables = %v, want the filter passed through", vars)
	}
}

// Zero-valued filters are omitted so the API returns every actor.
func TestReportActorsOmitsEmptyFilter(t *testing.T) {
	var vars map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		vars = req.Variables
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"data":{"reportData":{"report":{"masterData":{"actors":[]}}}}}`)
	}))
	defer srv.Close()
	client := &Client{gql: graphql.NewClient(srv.URL, srv.Client()), endpoint: srv.URL}

	if _, err := client.ReportActors(context.Background(), ReportActorsParams{Code: "abc"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := vars["actorType"]; ok {
		t.Errorf("actorType was sent despite being empty: %v", vars)
	}
	if _, ok := vars["actorSubType"]; ok {
		t.Errorf("actorSubType was sent despite being empty: %v", vars)
	}
}

func TestPhaseAt(t *testing.T) {
	// Deliberately unordered: the API does not promise sorted transitions.
	fight := ReportFight{PhaseTransitions: []PhaseTransition{
		{Id: 2, StartTime: 60_000},
		{Id: 1, StartTime: 10_000},
		{Id: 3, StartTime: 95_000},
	}}

	for _, tc := range []struct {
		at    float64
		want  int
		found bool
	}{
		{0, 0, false},     // before the pull
		{9_999, 0, false}, // before the first transition
		{10_000, 1, true}, // exactly on a transition
		{59_999, 1, true},
		{60_000, 2, true},
		{200_000, 3, true}, // after the last transition
	} {
		got, ok := fight.PhaseAt(tc.at)
		if ok != tc.found || (ok && got.Id != tc.want) {
			t.Errorf("PhaseAt(%v) = %d, %v; want %d, %v", tc.at, got.Id, ok, tc.want, tc.found)
		}
	}

	if _, ok := (&ReportFight{}).PhaseAt(1000); ok {
		t.Error("PhaseAt on a fight with no transitions = true, want false")
	}
}

func TestPhasesFor(t *testing.T) {
	r := &ReportWithFights{Phases: []EncounterPhases{
		{EncounterID: 3009, Phases: []PhaseMetadata{{Id: 1, Name: "Stage One"}}},
		{EncounterID: 3010, Phases: []PhaseMetadata{{Id: 1, Name: "Opening"}}},
	}}

	got := r.PhasesFor(3010)
	if len(got) != 1 || got[0].Name != "Opening" {
		t.Errorf("PhasesFor(3010) = %+v", got)
	}
	if got := r.PhasesFor(1); got != nil {
		t.Errorf("PhasesFor(unknown) = %+v, want nil", got)
	}
}
