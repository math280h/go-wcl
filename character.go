package warcraftlogs

import "context"

// Character looks up a character by its canonical ID. It returns [ErrNotFound]
// if no character matches.
func (c *Client) Character(ctx context.Context, id int) (*Character, error) {
	resp, err := getCharacterByID(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.CharacterData.Character)
}

// CharacterByName looks up a character by name, server slug, and server region
// (e.g. "us", "eu"). It returns [ErrNotFound] if no character matches.
func (c *Client) CharacterByName(ctx context.Context, name, serverSlug, serverRegion string) (*Character, error) {
	resp, err := getCharacterByName(ctx, c.gql, name, serverSlug, serverRegion)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.CharacterData.Character)
}
