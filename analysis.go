package warcraftlogs

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
)

// CharacterRef identifies a character by ID, or by name plus server slug and
// region (e.g. "us", "eu"). Populate one form or the other.
type CharacterRef struct {
	ID           int
	Name         string
	ServerSlug   string
	ServerRegion string
}

// ZoneRankingsParams filters character zone rankings. Zero fields are omitted.
type ZoneRankingsParams struct {
	Character          CharacterRef
	ZoneID             int
	Metric             CharacterPageRankingMetricType
	Difficulty         int
	Partition          int
	Role               RoleType
	Size               int
	ClassName          string
	SpecName           string
	ByBracket          bool
	Timeframe          RankingTimeframeType
	Compare            RankingCompareType
	IncludePrivateLogs bool
}

// CharacterZoneRankings returns a character's ranked performance across a zone
// as raw JSON. It returns [ErrNotFound] if no character matches.
func (c *Client) CharacterZoneRankings(ctx context.Context, p ZoneRankingsParams) (json.RawMessage, error) {
	resp, err := getCharacterZoneRankings(ctx, c.gql,
		p.Character.ID, p.Character.Name, p.Character.ServerSlug, p.Character.ServerRegion,
		p.ZoneID, p.Metric, p.Difficulty, p.Partition, p.Role, p.Size,
		p.ClassName, p.SpecName, p.ByBracket, p.Timeframe, p.Compare, p.IncludePrivateLogs)
	if err != nil {
		return nil, err
	}
	character, err := orNotFound(resp.CharacterData.Character)
	if err != nil {
		return nil, err
	}
	return character.ZoneRankings, nil
}

// EncounterRankingsParams filters character encounter rankings. Zero fields are
// omitted.
type EncounterRankingsParams struct {
	Character            CharacterRef
	EncounterID          int
	Metric               CharacterRankingMetricType
	Difficulty           int
	Partition            int
	Role                 RoleType
	Size                 int
	ClassName            string
	SpecName             string
	ByBracket            bool
	Timeframe            RankingTimeframeType
	Compare              RankingCompareType
	IncludeCombatantInfo bool
}

// CharacterEncounterRankings returns a character's ranked performance for a
// single encounter as raw JSON. It returns [ErrNotFound] if no character
// matches.
func (c *Client) CharacterEncounterRankings(ctx context.Context, p EncounterRankingsParams) (json.RawMessage, error) {
	resp, err := getCharacterEncounterRankings(ctx, c.gql,
		p.Character.ID, p.Character.Name, p.Character.ServerSlug, p.Character.ServerRegion,
		p.EncounterID, p.Metric, p.Difficulty, p.Partition, p.Role, p.Size,
		p.ClassName, p.SpecName, p.ByBracket, p.Timeframe, p.Compare, p.IncludeCombatantInfo)
	if err != nil {
		return nil, err
	}
	character, err := orNotFound(resp.CharacterData.Character)
	if err != nil {
		return nil, err
	}
	return character.EncounterRankings, nil
}

// ReportAnalysisParams filters the [Client.ReportTable] and [Client.ReportGraph]
// queries. Code is required; other zero fields are omitted. For arguments not
// exposed here, use [Client.Execute].
type ReportAnalysisParams struct {
	Code             string
	StartTime        float64
	EndTime          float64
	FightIDs         []int
	EncounterID      int
	Difficulty       int
	KillType         KillType
	HostilityType    HostilityType
	SourceID         int
	TargetID         int
	AbilityID        float64
	FilterExpression string
	ViewBy           ViewType
}

// ReportTable returns a report analysis table as raw JSON. It returns
// [ErrNotFound] if no report matches.
func (c *Client) ReportTable(ctx context.Context, dataType TableDataType, p ReportAnalysisParams) (json.RawMessage, error) {
	resp, err := getReportTable(ctx, c.gql, p.Code, dataType,
		p.StartTime, p.EndTime, p.FightIDs, p.EncounterID, p.Difficulty,
		p.KillType, p.HostilityType, p.SourceID, p.TargetID, p.AbilityID,
		p.FilterExpression, p.ViewBy)
	if err != nil {
		return nil, err
	}
	report, err := orNotFound(resp.ReportData.Report)
	if err != nil {
		return nil, err
	}
	return report.Table, nil
}

// ReportGraph returns report graph data as raw JSON. It returns [ErrNotFound]
// if no report matches.
func (c *Client) ReportGraph(ctx context.Context, dataType GraphDataType, p ReportAnalysisParams) (json.RawMessage, error) {
	resp, err := getReportGraph(ctx, c.gql, p.Code, dataType,
		p.StartTime, p.EndTime, p.FightIDs, p.EncounterID, p.Difficulty,
		p.KillType, p.HostilityType, p.SourceID, p.TargetID, p.AbilityID,
		p.FilterExpression, p.ViewBy)
	if err != nil {
		return nil, err
	}
	report, err := orNotFound(resp.ReportData.Report)
	if err != nil {
		return nil, err
	}
	return report.Graph, nil
}

// ReportEventsParams filters the [Client.ReportEvents] query. Code is required;
// other zero fields are omitted.
type ReportEventsParams struct {
	Code             string
	StartTime        float64
	EndTime          float64
	FightIDs         []int
	EncounterID      int
	Difficulty       int
	KillType         KillType
	HostilityType    HostilityType
	SourceID         int
	TargetID         int
	AbilityID        float64
	FilterExpression string
	IncludeResources bool
	Limit            int
}

// EventPage is one page of report events. NextPageTimestamp is zero when there
// are no further pages; otherwise pass it as StartTime to fetch the next page.
type EventPage struct {
	Data              json.RawMessage
	NextPageTimestamp float64
}

// ReportEvents returns a page of report events as raw JSON. It returns
// [ErrNotFound] if no report matches.
func (c *Client) ReportEvents(ctx context.Context, dataType EventDataType, p ReportEventsParams) (EventPage, error) {
	resp, err := getReportEvents(ctx, c.gql, p.Code, dataType,
		p.StartTime, p.EndTime, p.FightIDs, p.EncounterID, p.Difficulty,
		p.KillType, p.HostilityType, p.SourceID, p.TargetID, p.AbilityID,
		p.FilterExpression, p.IncludeResources, p.Limit)
	if err != nil {
		return EventPage{}, err
	}
	report, err := orNotFound(resp.ReportData.Report)
	if err != nil {
		return EventPage{}, err
	}
	events := report.Events
	return EventPage{Data: events.Data, NextPageTimestamp: events.NextPageTimestamp}, nil
}

// ReportEventsAll iterates every event matching p, following
// [EventPage.NextPageTimestamp] across pages and yielding one event at a time.
// Events arrive as raw JSON objects for the caller to decode.
//
// Iteration stops at the first error, which is yielded with a nil event. Stop
// early by breaking out of the range loop; no further requests are made.
//
//	for e, err := range client.ReportEventsAll(ctx, dataType, params) {
//		if err != nil {
//			return err
//		}
//		// ... decode e ...
//	}
//
// Like [Client.ReportEvents], this requires either FightIDs or an explicit
// StartTime and EndTime range. It does not modify p.
func (c *Client) ReportEventsAll(ctx context.Context, dataType EventDataType, p ReportEventsParams) iter.Seq2[json.RawMessage, error] {
	return func(yield func(json.RawMessage, error) bool) {
		for {
			page, err := c.ReportEvents(ctx, dataType, p)
			if err != nil {
				yield(nil, err)
				return
			}

			var events []json.RawMessage
			if len(page.Data) > 0 {
				if err := json.Unmarshal(page.Data, &events); err != nil {
					yield(nil, fmt.Errorf("warcraftlogs: decoding events at %v: %w", p.StartTime, err))
					return
				}
			}
			for _, e := range events {
				if !yield(e, nil) {
					return
				}
			}

			if page.NextPageTimestamp == 0 {
				return
			}
			if page.NextPageTimestamp <= p.StartTime {
				yield(nil, fmt.Errorf("%w: %v did not advance past %v",
					ErrPageNotAdvancing, page.NextPageTimestamp, p.StartTime))
				return
			}
			p.StartTime = page.NextPageTimestamp
		}
	}
}

// ReportRankingsParams filters the [Client.ReportRankings] query. Code is
// required; other zero fields are omitted.
type ReportRankingsParams struct {
	Code         string
	EncounterID  int
	Difficulty   int
	FightIDs     []int
	PlayerMetric ReportRankingMetricType
	Compare      RankingCompareType
	Timeframe    RankingTimeframeType
}

// ReportRankings returns a report's rankings as raw JSON. It returns
// [ErrNotFound] if no report matches.
func (c *Client) ReportRankings(ctx context.Context, p ReportRankingsParams) (json.RawMessage, error) {
	resp, err := getReportRankings(ctx, c.gql, p.Code, p.EncounterID, p.Difficulty,
		p.FightIDs, p.PlayerMetric, p.Compare, p.Timeframe)
	if err != nil {
		return nil, err
	}
	report, err := orNotFound(resp.ReportData.Report)
	if err != nil {
		return nil, err
	}
	return report.Rankings, nil
}

// PlayerDetailsParams filters the [Client.ReportPlayerDetails] query. Code is
// required; other zero fields are omitted.
type PlayerDetailsParams struct {
	Code                 string
	StartTime            float64
	EndTime              float64
	FightIDs             []int
	EncounterID          int
	Difficulty           int
	KillType             KillType
	IncludeCombatantInfo bool
	Translate            bool
}

// ReportPlayerDetails returns per-player detail for a report as raw JSON. It
// returns [ErrNotFound] if no report matches.
func (c *Client) ReportPlayerDetails(ctx context.Context, p PlayerDetailsParams) (json.RawMessage, error) {
	resp, err := getReportPlayerDetails(ctx, c.gql, p.Code, p.StartTime, p.EndTime,
		p.FightIDs, p.EncounterID, p.Difficulty, p.KillType, p.IncludeCombatantInfo, p.Translate)
	if err != nil {
		return nil, err
	}
	report, err := orNotFound(resp.ReportData.Report)
	if err != nil {
		return nil, err
	}
	return report.PlayerDetails, nil
}
