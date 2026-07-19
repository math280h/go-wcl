package warcraftlogs

import "context"

// Zones returns the zones of an expansion, or of all expansions when
// expansionID is zero.
func (c *Client) Zones(ctx context.Context, expansionID int) ([]Zone, error) {
	resp, err := getZones(ctx, c.gql, expansionID)
	if err != nil {
		return nil, err
	}
	return resp.WorldData.Zones, nil
}

// Zone looks up a zone by its ID. It returns [ErrNotFound] if no zone matches.
func (c *Client) Zone(ctx context.Context, id int) (*Zone, error) {
	resp, err := getZone(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.WorldData.Zone)
}

// Encounter looks up an encounter by its ID. It returns [ErrNotFound] if no
// encounter matches.
func (c *Client) Encounter(ctx context.Context, id int) (*Encounter, error) {
	resp, err := getEncounter(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.WorldData.Encounter)
}

// Expansions returns all expansions.
func (c *Client) Expansions(ctx context.Context) ([]Expansion, error) {
	resp, err := getExpansions(ctx, c.gql)
	if err != nil {
		return nil, err
	}
	return resp.WorldData.Expansions, nil
}

// Expansion looks up an expansion by its ID. It returns [ErrNotFound] if no
// expansion matches.
func (c *Client) Expansion(ctx context.Context, id int) (*Expansion, error) {
	resp, err := getExpansion(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.WorldData.Expansion)
}

// Regions returns all regions.
func (c *Client) Regions(ctx context.Context) ([]Region, error) {
	resp, err := getRegions(ctx, c.gql)
	if err != nil {
		return nil, err
	}
	return resp.WorldData.Regions, nil
}

// Server looks up a server by its ID. It returns [ErrNotFound] if no server
// matches.
func (c *Client) Server(ctx context.Context, id int) (*Server, error) {
	resp, err := getServerByID(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.WorldData.Server)
}

// ServerBySlug looks up a server by its region abbreviation (e.g. "US") and
// slug. It returns [ErrNotFound] if no server matches.
func (c *Client) ServerBySlug(ctx context.Context, region, slug string) (*Server, error) {
	resp, err := getServerBySlug(ctx, c.gql, region, slug)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.WorldData.Server)
}
