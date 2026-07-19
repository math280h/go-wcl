package main

import (
	"context"
	"fmt"
	"time"

	warcraftlogs "github.com/math280h/go-wcl"
)

func describeReport(ctx context.Context, client *warcraftlogs.Client, code string) error {
	report, err := client.Report(ctx, code, false)
	if err != nil {
		return err
	}
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
	return nil
}

// summarizeFights prints a per-boss pull count and returns the last kill.
func summarizeFights(fights []warcraftlogs.ReportFight) (lastKill *warcraftlogs.ReportFight) {
	type tally struct {
		pulls, kills int
		best         float64
	}
	order := []string{}
	byBoss := map[string]*tally{}

	for i, f := range fights {
		if f.EncounterID == 0 {
			continue // trash
		}
		t := byBoss[f.Name]
		if t == nil {
			t = &tally{best: 100}
			byBoss[f.Name] = t
			order = append(order, f.Name)
		}
		t.pulls++
		if f.Kill {
			t.kills++
			lastKill = &fights[i]
			continue
		}
		if f.FightPercentage < t.best {
			t.best = f.FightPercentage
		}
	}

	fmt.Printf("\n== encounters ==\n")
	for _, name := range order {
		t := byBoss[name]
		status := fmt.Sprintf("wiped, best %.1f%%", t.best)
		if t.kills > 0 {
			status = "killed"
		}
		fmt.Printf("%-32s %2d pulls  %s\n", name, t.pulls, status)
	}
	return lastKill
}
