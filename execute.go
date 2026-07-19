package warcraftlogs

import (
	"context"

	"github.com/Khan/genqlient/graphql"
)

// Execute runs a GraphQL query and decodes the "data" field of the response
// into out, which must be a pointer. Use it for operations not covered by the
// generated methods.
func (c *Client) Execute(ctx context.Context, query string, variables map[string]any, out any) error {
	return c.gql.MakeRequest(ctx, &graphql.Request{
		Query:     query,
		Variables: variables,
	}, &graphql.Response{Data: out})
}
