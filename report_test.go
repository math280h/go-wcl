package warcraftlogs

import "testing"

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
