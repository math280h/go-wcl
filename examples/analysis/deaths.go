package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	warcraftlogs "github.com/math280h/go-wcl"
)

// deathEvent is one element of the array ReportEvents returns for
// EventDataTypeDeaths. TargetID and KillingAbilityGameID resolve through
// ReportMasterData.
type deathEvent struct {
	Timestamp            float64 `json:"timestamp"`
	Fight                int     `json:"fight"`
	TargetID             int     `json:"targetID"`
	KillingAbilityGameID int64   `json:"killingAbilityGameID"`
}

func deaths(ctx context.Context, client *warcraftlogs.Client, code string, fights []warcraftlogs.ReportFight) error {
	var bossPulls []int
	for _, f := range fights {
		if f.EncounterID != 0 {
			bossPulls = append(bossPulls, f.Id)
		}
	}
	if len(bossPulls) == 0 {
		return nil
	}
	events, err := allDeaths(ctx, client, code, bossPulls)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return nil
	}

	// Pick the pull that cost the most deaths.
	perFight := map[int]int{}
	for _, e := range events {
		perFight[e.Fight]++
	}
	var fight warcraftlogs.ReportFight
	for _, f := range fights {
		if perFight[f.Id] > perFight[fight.Id] {
			fight = f
		}
	}

	master, err := client.ReportMasterData(ctx, code, false)
	if err != nil {
		return err
	}
	actors := make(map[int]string, len(master.Actors))
	for _, a := range master.Actors {
		actors[a.Id] = a.Name
	}
	abilities := make(map[int64]string, len(master.Abilities))
	for _, a := range master.Abilities {
		abilities[int64(a.GameID)] = a.Name
	}

	outcome := "wipe"
	if fight.Kill != nil && *fight.Kill {
		outcome = "kill"
	}
	fmt.Printf("\n== deaths: %s (%s, %d of %d in the report) ==\n",
		fight.Name, outcome, perFight[fight.Id], len(events))
	shown := 0
	for _, e := range events {
		if e.Fight != fight.Id {
			continue
		}
		if shown == 5 {
			fmt.Printf("... and %d more\n", perFight[fight.Id]-shown)
			break
		}
		shown++
		at := time.Duration(e.Timestamp-fight.StartTime) * time.Millisecond
		killer := abilities[e.KillingAbilityGameID]
		if killer == "" {
			killer = "unknown"
		}
		fmt.Printf("%6s  %-20s killed by %s\n", at.Round(time.Second), actors[e.TargetID], killer)
	}
	return nil
}

// allDeaths pages through the death events of the given fights.
// NextPageTimestamp is zero on the last page; otherwise it is the StartTime of
// the next one.
func allDeaths(ctx context.Context, client *warcraftlogs.Client, code string, fightIDs []int) ([]deathEvent, error) {
	params := warcraftlogs.ReportEventsParams{
		Code:     code,
		FightIDs: fightIDs,
	}
	var all []deathEvent
	for raw, err := range client.ReportEventsAll(ctx, warcraftlogs.EventDataTypeDeaths, params) {
		if err != nil {
			return nil, err
		}
		var e deathEvent
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("decode death event: %w", err)
		}
		all = append(all, e)
	}
	return all, nil
}
