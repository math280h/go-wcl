package warcraftlogs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// page builds a getReportEvents response carrying data and a next cursor.
func page(data string, next float64) string {
	return fmt.Sprintf(
		`{"data":{"reportData":{"report":{"events":{"data":%s,"nextPageTimestamp":%v}}}}}`,
		data, next)
}

func collect(t *testing.T, seq func(func(json.RawMessage, error) bool)) ([]string, error) {
	t.Helper()
	var got []string
	var err error
	for e, e2 := range seq {
		if e2 != nil {
			err = e2
			break
		}
		got = append(got, string(e))
	}
	return got, err
}

func TestReportEventsAllFollowsPages(t *testing.T) {
	client, srv := newStubGQL(t,
		page(`[{"n":1},{"n":2}]`, 500),
		page(`[{"n":3}]`, 900),
		page(`[{"n":4}]`, 0),
	)

	got, err := collect(t, client.ReportEventsAll(context.Background(), EventDataTypeDeaths,
		ReportEventsParams{Code: "abc", FightIDs: []int{1}}))
	if err != nil {
		t.Fatal(err)
	}

	want := []string{`{"n":1}`, `{"n":2}`, `{"n":3}`, `{"n":4}`}
	if len(got) != len(want) {
		t.Fatalf("got %d events, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d = %s, want %s", i, got[i], want[i])
		}
	}
	// The cursor from each page becomes the next request's StartTime.
	if want := []float64{0, 500, 900}; !equalFloats(srv.variable("startTime"), want) {
		t.Errorf("startTimes = %v, want %v", srv.variable("startTime"), want)
	}
}

func TestReportEventsAllStopsOnNonAdvancingCursor(t *testing.T) {
	client, srv := newStubGQL(t,
		page(`[{"n":1}]`, 500),
		page(`[{"n":2}]`, 500), // same cursor again
	)

	got, err := collect(t, client.ReportEventsAll(context.Background(), EventDataTypeDeaths,
		ReportEventsParams{Code: "abc", FightIDs: []int{1}}))
	if !errors.Is(err, ErrPageNotAdvancing) {
		t.Fatalf("err = %v, want ErrPageNotAdvancing", err)
	}
	if len(got) != 2 {
		t.Errorf("got %d events, want the 2 from before the stall", len(got))
	}
	if srv.calls() != 2 {
		t.Errorf("calls = %d, want 2 (must not keep re-requesting)", srv.calls())
	}
}

func TestReportEventsAllStopsOnBreak(t *testing.T) {
	client, srv := newStubGQL(t, page(`[{"n":1},{"n":2}]`, 500))

	count := 0
	for _, err := range client.ReportEventsAll(context.Background(), EventDataTypeDeaths,
		ReportEventsParams{Code: "abc", FightIDs: []int{1}}) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		break
	}
	if count != 1 {
		t.Errorf("yielded %d events after break, want 1", count)
	}
	if srv.calls() != 1 {
		t.Errorf("calls = %d, want 1 (break must not fetch the next page)", srv.calls())
	}
}

func TestReportEventsAllPropagatesErrors(t *testing.T) {
	client, _ := newStubGQL(t,
		`{"errors":[{"message":"This report does not exist."}]}`,
	)

	got, err := collect(t, client.ReportEventsAll(context.Background(), EventDataTypeDeaths,
		ReportEventsParams{Code: "abc", FightIDs: []int{1}}))
	if err == nil {
		t.Fatal("expected an error")
	}
	if len(GraphQLErrors(err)) == 0 {
		t.Errorf("err = %v, want a GraphQL error", err)
	}
	if got != nil {
		t.Errorf("got %v, want no events", got)
	}
}

// A null data field is not an error; it just ends the iteration.
func TestReportEventsAllHandlesEmptyPage(t *testing.T) {
	client, _ := newStubGQL(t, page(`null`, 0))

	got, err := collect(t, client.ReportEventsAll(context.Background(), EventDataTypeDeaths,
		ReportEventsParams{Code: "abc", FightIDs: []int{1}}))
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("got %v, want no events", got)
	}
}

func TestEncounterLeaderboard(t *testing.T) {
	client, srv := newStubGQL(t,
		`{"data":{"worldData":{"encounter":{"characterRankings":{"rankings":[{"name":"Xanttcha"}]}}}}}`)

	raw, err := client.EncounterLeaderboard(context.Background(), EncounterLeaderboardParams{
		EncounterID: 3009,
		ClassName:   "Mage", SpecName: "Fire", Metric: CharacterRankingMetricTypeDps,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Xanttcha") {
		t.Errorf("rankings = %s", raw)
	}
	if srv.last()["id"] != float64(3009) || srv.last()["className"] != "Mage" || srv.last()["specName"] != "Fire" {
		t.Errorf("variables = %v", srv.last())
	}
	for _, name := range []string{"difficulty", "page", "serverSlug", "filter"} {
		if srv.sent(name) {
			t.Errorf("%s was sent despite being zero: %v", name, srv.last())
		}
	}
}

func TestEncounterLeaderboardNotFound(t *testing.T) {
	client, _ := newStubGQL(t, `{"data":{"worldData":{"encounter":null}}}`)

	_, err := client.EncounterLeaderboard(context.Background(), EncounterLeaderboardParams{EncounterID: 1})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func equalFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
