package warcraftlogs

import (
	"context"
	"errors"

	"github.com/Khan/genqlient/graphql"
)

const (
	// ClientEndpoint is the public API, accessed with the client-credentials flow.
	ClientEndpoint = "https://www.warcraftlogs.com/api/v2/client"
	// UserEndpoint is the private API, accessed with the authorization-code or PKCE flow.
	UserEndpoint = "https://www.warcraftlogs.com/api/v2/user"

	// AuthorizeURL is the OAuth 2.0 authorization endpoint.
	AuthorizeURL = "https://www.warcraftlogs.com/oauth/authorize"
	// TokenURL is the OAuth 2.0 token endpoint.
	TokenURL = "https://www.warcraftlogs.com/oauth/token"
)

// ErrNoCredentials is returned by [New] when no authentication is configured.
var ErrNoCredentials = errors.New("warcraftlogs: no credentials configured (use WithClientCredentials, WithTokenSource, or WithHTTPClient)")

// Client is a Warcraft Logs API client. It is safe for concurrent use.
type Client struct {
	gql      graphql.Client
	endpoint string
}

// New creates a Client. Provide authentication with [WithClientCredentials],
// [WithTokenSource], or [WithHTTPClient]. Canceling ctx does not affect the
// returned Client.
func New(ctx context.Context, opts ...Option) (*Client, error) {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	httpClient, err := o.httpClientFor(ctx)
	if err != nil {
		return nil, err
	}

	return &Client{
		gql:      graphql.NewClient(o.endpoint, httpClient),
		endpoint: o.endpoint,
	}, nil
}

// Endpoint returns the GraphQL endpoint the client targets.
func (c *Client) Endpoint() string { return c.endpoint }

// GraphQL returns the underlying genqlient client for use with generated
// operations or custom tooling.
func (c *Client) GraphQL() graphql.Client { return c.gql }
