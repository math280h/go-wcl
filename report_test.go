package warcraftlogs

import (
	"context"
	"testing"
)

func TestReportActorsSendsFilterAndUnwraps(t *testing.T) {
	client, srv := newStubGQL(t, `{"data":{"reportData":{"report":{"masterData":{"actors":[
		{"id":1,"name":"Zesyis","type":"Player","subType":"DeathKnight"}]}}}}}`)

	actors, err := client.ReportActors(context.Background(), ReportActorsParams{
		Code: "abc", Type: ActorPlayer, SubType: "DeathKnight",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(actors) != 1 || actors[0].Name != "Zesyis" {
		t.Fatalf("actors = %+v", actors)
	}
	if srv.last()["actorType"] != "Player" || srv.last()["actorSubType"] != "DeathKnight" {
		t.Errorf("variables = %v, want the filter passed through", srv.last())
	}
}

// Zero-valued filters are omitted so the API returns every actor.
func TestReportActorsOmitsEmptyFilter(t *testing.T) {
	client, srv := newStubGQL(t, `{"data":{"reportData":{"report":{"masterData":{"actors":[]}}}}}`)

	if _, err := client.ReportActors(context.Background(), ReportActorsParams{Code: "abc"}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"actorType", "actorSubType"} {
		if srv.sent(name) {
			t.Errorf("%s was sent despite being empty: %v", name, srv.last())
		}
	}
}

func TestReportsUnwrapsThePage(t *testing.T) {
	client, srv := newStubGQL(t, `{"data":{"reportData":{"reports":{
		"total":42,"perPage":10,"currentPage":2,"lastPage":5,"hasMorePages":true,
		"from":11,"to":20,
		"data":[{"code":"aaa","title":"Night One"},{"code":"bbb","title":"Night Two"}]}}}}`)

	page, err := client.Reports(context.Background(), ReportsParams{
		GuildName: "Skill Issue", GuildServerSlug: "tarren-mill", GuildServerRegion: "eu", Page: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 42 || page.PerPage != 10 || page.CurrentPage != 2 || page.LastPage != 5 {
		t.Errorf("envelope = %+v", page)
	}
	if !page.HasMorePages {
		t.Error("HasMorePages = false, want true")
	}
	if len(page.Data) != 2 || page.Data[0].Code != "aaa" {
		t.Errorf("data = %+v", page.Data)
	}
	if srv.last()["guildName"] != "Skill Issue" || srv.last()["guildServerRegion"] != "eu" {
		t.Errorf("variables = %v", srv.last())
	}
	// Unset guild identifiers must not narrow the query.
	for _, name := range []string{"guildID", "guildTagID", "userID", "zoneID"} {
		if srv.sent(name) {
			t.Errorf("%s was sent despite being zero: %v", name, srv.last())
		}
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
