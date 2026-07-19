package main

import (
	"fmt"
	"time"

	warcraftlogs "github.com/math280h/go-wcl"
)

func describeReport(code string, report *warcraftlogs.ReportWithFights) {
	start := time.UnixMilli(int64(report.StartTime))
	end := time.UnixMilli(int64(report.EndTime))

	fmt.Printf("== report %s ==\n", code)
	fmt.Printf("title:  %s\n", report.Title)
	fmt.Printf("zone:   %s\n", report.Zone.Name)
	if report.Guild.Name != "" {
		fmt.Printf("guild:  %s\n", report.Guild.Name)
	}
	fmt.Printf("owner:  %s\n", report.Owner.Name)
	fmt.Printf("date:   %s (%s)\n", start.Format(time.DateOnly), end.Sub(start).Round(time.Minute))
}

// summarizeFights prints a per-boss pull count and returns the last kill.
func summarizeFights(report *warcraftlogs.ReportWithFights) (lastKill *warcraftlogs.ReportFight) {
	type tally struct {
		pulls, kills int
		best         float64
		bestPhase    string
	}
	order := []string{}
	byBoss := map[string]*tally{}

	for i, f := range report.Fights {
		// Kill is null on trash, false on a wipe. EncounterID is 0 for both.
		if f.Kill == nil {
			continue
		}
		t := byBoss[f.Name]
		if t == nil {
			t = &tally{best: 100}
			byBoss[f.Name] = t
			order = append(order, f.Name)
		}
		t.pulls++
		if *f.Kill {
			t.kills++
			lastKill = &report.Fights[i]
			continue
		}
		if f.FightPercentage != nil && *f.FightPercentage < t.best {
			t.best = *f.FightPercentage
			t.bestPhase = phaseName(report, &report.Fights[i])
		}
	}

	fmt.Printf("\n== encounters ==\n")
	for _, name := range order {
		t := byBoss[name]
		status := fmt.Sprintf("wiped, best %.1f%%", t.best)
		if t.bestPhase != "" {
			status += " in " + t.bestPhase
		}
		if t.kills > 0 {
			status = "killed"
		}
		fmt.Printf("%-32s %2d pulls  %s\n", name, t.pulls, status)
	}
	return lastKill
}

// phaseName resolves the phase a fight ended in by joining its last observed
// transition against the report's phase metadata.
func phaseName(report *warcraftlogs.ReportWithFights, f *warcraftlogs.ReportFight) string {
	pt, ok := f.PhaseAt(f.EndTime)
	if !ok {
		return ""
	}
	for _, p := range report.PhasesFor(f.EncounterID) {
		if p.Id != pt.Id {
			continue
		}
		if p.IsIntermission != nil && *p.IsIntermission {
			return p.Name + " (intermission)"
		}
		return p.Name
	}
	return ""
}
