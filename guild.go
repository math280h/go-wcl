package warcraftlogs

import "context"

// Guild looks up a guild by its ID. It returns [ErrNotFound] if no guild
// matches.
func (c *Client) Guild(ctx context.Context, id int) (*Guild, error) {
	resp, err := getGuildByID(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.GuildData.Guild)
}

// GuildByName looks up a guild by name, server slug, and server region
// (e.g. "us", "eu"). It returns [ErrNotFound] if no guild matches.
func (c *Client) GuildByName(ctx context.Context, name, serverSlug, serverRegion string) (*Guild, error) {
	resp, err := getGuildByName(ctx, c.gql, name, serverSlug, serverRegion)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.GuildData.Guild)
}
