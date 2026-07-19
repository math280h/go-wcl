package warcraftlogs

import "context"

// CurrentUser returns the authenticated user. It requires [UserEndpoint] and a
// token from the authorization-code or PKCE flow, and returns [ErrNotFound]
// otherwise.
func (c *Client) CurrentUser(ctx context.Context) (*CurrentUser, error) {
	resp, err := getCurrentUser(ctx, c.gql)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.UserData.CurrentUser)
}

// User looks up a user by ID. It returns [ErrNotFound] if no user matches.
// Avatar and battle tag are only readable through [Client.CurrentUser].
func (c *Client) User(ctx context.Context, id int) (*User, error) {
	resp, err := getUser(ctx, c.gql, id)
	if err != nil {
		return nil, err
	}
	return orNotFound(resp.UserData.User)
}
