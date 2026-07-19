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

// ReportFightsParams filters the fights returned by [Client.ReportFights]. Code
// is required; other zero fields are omitted from the query.
type ReportFightsParams struct {
	Code        string
	EncounterID int
	Difficulty  int
	KillType    KillType
	FightIDs    []int
	Translate   bool
}

// ReportFights returns the fights of a report. It returns [ErrNotFound] if no
// report matches.
func (c *Client) ReportFights(ctx context.Context, p ReportFightsParams) ([]ReportFight, error) {
	resp, err := getReportFights(ctx, c.gql, p.Code, p.EncounterID, p.Difficulty, p.KillType, p.FightIDs, p.Translate)
	if err != nil {
		return nil, err
	}
	report, err := orNotFound(resp.ReportData.Report)
	if err != nil {
		return nil, err
	}
	return report.Fights, nil
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
