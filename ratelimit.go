package warcraftlogs

import "context"

// RateLimit returns the API point budget for the authenticated key. The budget
// resets on a rolling one-hour window; costs scale with the amount of data a
// query requests.
func (c *Client) RateLimit(ctx context.Context) (RateLimit, error) {
	resp, err := getRateLimit(ctx, c.gql)
	if err != nil {
		return RateLimit{}, err
	}
	return resp.RateLimitData, nil
}
