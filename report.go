package warcraftlogs

import "context"

// Report looks up a report by its code. It returns [ErrNotFound] if no report
// matches. Set allowUnlisted to access unlisted reports the authenticated key
// may view.
func (c *Client) Report(ctx context.Context, code string, allowUnlisted bool) (*Report, error) {
	resp, err := getReport(ctx, c.gql, code, allowUnlisted)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.ReportData.Report)
}

// ReportWithFightsParams filters the fights returned by
// [Client.ReportWithFights]. Code is required; other zero fields are omitted
// from the query.
type ReportWithFightsParams struct {
	Code          string
	AllowUnlisted bool
	EncounterID   int
	Difficulty    int
	KillType      KillType
	FightIDs      []int
	Translate     bool
}

// ReportWithFights is a report header together with its fights and the phase
// metadata of every boss encounter the report observed.
type ReportWithFights struct {
	Report
	Fights []ReportFight
	Phases []EncounterPhases
}

// PhasesFor returns the phase metadata for an encounter, or nil if the report
// carries none. [PhaseMetadata.Id] lines up with [PhaseTransition.Id].
func (r *ReportWithFights) PhasesFor(encounterID int) []PhaseMetadata {
	for _, ep := range r.Phases {
		if ep.EncounterID == encounterID {
			return ep.Phases
		}
	}
	return nil
}

// ReportWithFights returns a report's header, fights and encounter phases in a
// single request. Use [Client.Report] when the header is all you need. It
// returns [ErrNotFound] if no report matches.
func (c *Client) ReportWithFights(ctx context.Context, p ReportWithFightsParams) (*ReportWithFights, error) {
	resp, err := getReportWithFights(ctx, c.gql, p.Code, p.AllowUnlisted, p.EncounterID, p.Difficulty, p.KillType, p.FightIDs, p.Translate)
	if err != nil {
		return nil, err
	}
	report, err := orNotFound(resp.ReportData.Report)
	if err != nil {
		return nil, err
	}
	return &ReportWithFights{
		Report: report.Report,
		Fights: report.Fights,
		Phases: report.Phases,
	}, nil
}

// PhaseAt returns the phase the fight was in at report-relative timestamp t,
// and whether one was found. It reports false for a timestamp before the first
// transition, and for fights with no phase data. Join [PhaseTransition.Id]
// against [ReportWithFights.PhasesFor] to resolve the name.
//
// Prefer this to [ReportFight.LastPhase], which numbers normal phases and
// intermissions separately and so only resolves alongside
// LastPhaseIsIntermission. A fight may also re-enter a phase it has already
// been in.
func (f *ReportFight) PhaseAt(t float64) (PhaseTransition, bool) {
	var cur PhaseTransition
	var found bool
	for _, pt := range f.PhaseTransitions {
		if pt.StartTime <= t && (!found || pt.StartTime > cur.StartTime) {
			cur, found = pt, true
		}
	}
	return cur, found
}

// ReportsParams selects the reports returned by [Client.Reports]. Zero fields
// are omitted from the query. Identify a guild either by GuildID or by the
// GuildName, GuildServerSlug and GuildServerRegion trio; GuildTagID takes
// precedence over both. UserID lists a user's personal logs instead.
type ReportsParams struct {
	GuildID           int
	GuildName         string
	GuildServerSlug   string
	GuildServerRegion string
	GuildTagID        int
	UserID            int
	ZoneID            int
	GameZoneID        int
	StartTime         float64
	EndTime           float64
	Limit             int
	Page              int
}

// Reports lists uploaded reports for a guild or user. Advance through the
// result by incrementing Page while [ReportPage.HasMorePages] is true. Limit
// defaults to 100, which is also the maximum.
func (c *Client) Reports(ctx context.Context, p ReportsParams) (ReportPage, error) {
	resp, err := getReports(ctx, c.gql, p.GuildID, p.GuildName, p.GuildServerSlug,
		p.GuildServerRegion, p.GuildTagID, p.UserID, p.ZoneID, p.GameZoneID,
		p.StartTime, p.EndTime, p.Limit, p.Page)
	if err != nil {
		return ReportPage{}, err
	}
	return resp.ReportData.Reports, nil
}

// Actor types and sub-types accepted by [ReportActorsParams]. The API matches
// these case-sensitively against its own actor classification.
const (
	ActorPlayer = "Player"
	ActorNPC    = "NPC"
	ActorPet    = "Pet"
	ActorBoss   = "Boss"
)

// ReportActorsParams filters the actors returned by [Client.ReportActors]. Code
// is required; other zero fields are omitted from the query. Type selects
// players, NPCs or pets; SubType selects a player's class or an NPC's
// classification.
type ReportActorsParams struct {
	Code      string
	Type      string
	SubType   string
	Translate bool
}

// ReportActors returns a report's actors, filtered by type and sub-type. It is
// the narrow form of [Client.ReportMasterData], which always returns every
// actor alongside the full ability table. It returns [ErrNotFound] if no report
// matches.
func (c *Client) ReportActors(ctx context.Context, p ReportActorsParams) ([]ReportActor, error) {
	resp, err := getReportActors(ctx, c.gql, p.Code, p.Type, p.SubType, p.Translate)
	if err != nil {
		return nil, err
	}
	report, err := orNotFound(resp.ReportData.Report)
	if err != nil {
		return nil, err
	}
	return report.MasterData.Actors, nil
}

// ReportMasterData returns the actors and abilities referenced by a report. Set
// translate to localize names to the request locale. It returns [ErrNotFound]
// if no report matches.
func (c *Client) ReportMasterData(ctx context.Context, code string, translate bool) (ReportMasterData, error) {
	resp, err := getReportMasterData(ctx, c.gql, code, translate)
	if err != nil {
		return ReportMasterData{}, err
	}
	report, err := orNotFound(resp.ReportData.Report)
	if err != nil {
		return ReportMasterData{}, err
	}
	return report.MasterData, nil
}
