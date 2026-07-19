package warcraftlogs

import "context"

// Ability looks up a game ability by its ID. It returns [ErrNotFound] if no
// ability matches.
func (c *Client) Ability(ctx context.Context, id int) (*GameAbility, error) {
	resp, err := getAbility(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.GameData.Ability)
}

// Item looks up a game item by its ID. It returns [ErrNotFound] if no item
// matches.
func (c *Client) Item(ctx context.Context, id int) (*GameItem, error) {
	resp, err := getItem(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.GameData.Item)
}

// NPC looks up a game NPC by its ID. It returns [ErrNotFound] if no NPC matches.
func (c *Client) NPC(ctx context.Context, id int) (*GameNPC, error) {
	resp, err := getNPC(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.GameData.Npc)
}

// Classes returns the playable classes, optionally filtered by faction and
// zone. A zero filter is omitted.
func (c *Client) Classes(ctx context.Context, factionID, zoneID int) ([]GameClass, error) {
	resp, err := getClasses(ctx, c.gql, factionID, zoneID)
	if err != nil {
		return nil, err
	}
	return resp.GameData.Classes, nil
}
