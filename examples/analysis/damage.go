package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	warcraftlogs "github.com/math280h/go-wcl"
)

// damageTable is the payload of ReportTable for TableDataTypeDamagedone.
type damageTable struct {
	Data struct {
		Entries []damageEntry `json:"entries"`
	} `json:"data"`
}

type damageEntry struct {
	Name      string `json:"name"`
	Type      string `json:"type"` // class, e.g. "Warlock"
	ItemLevel int    `json:"itemLevel"`
	Total     int64  `json:"total"`
}

func topDamage(ctx context.Context, client *warcraftlogs.Client, code string, fight warcraftlogs.ReportFight) error {
	raw, err := client.ReportTable(ctx, warcraftlogs.TableDataTypeDamagedone,
		warcraftlogs.ReportAnalysisParams{Code: code, FightIDs: []int{fight.Id}})
	if err != nil {
		return err
	}
	var table damageTable
	if err := json.Unmarshal(raw, &table); err != nil {
		return fmt.Errorf("decode damage table: %w", err)
	}

	entries := table.Data.Entries
	sort.Slice(entries, func(i, j int) bool { return entries[i].Total > entries[j].Total })

	seconds := (fight.EndTime - fight.StartTime) / 1000
	fmt.Printf("\n== top damage: %s (%s kill) ==\n", fight.Name, time.Duration(seconds)*time.Second)
	for i, e := range entries {
		if i >= 5 {
			break
		}
		fmt.Printf("%d. %-20s %-14s ilvl %d  %8.1fk dps\n",
			i+1, e.Name, e.Type, e.ItemLevel, float64(e.Total)/seconds/1000)
	}
	return nil
}
