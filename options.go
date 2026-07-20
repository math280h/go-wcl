package warcraftlogs

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const defaultUserAgent = "go-wcl (+https://github.com/math280h/go-wcl)"

var discardLogger = slog.New(slog.DiscardHandler)

// Option configures a [Client].
type Option func(*options)

type options struct {
	endpoint      string
	tokenURL      string
	userAgent     string
	maxRetries    int
	timeout       time.Duration
	scopes        []string
	clientID      string
	clientSecret  string
	tokenSource   oauth2.TokenSource
	httpClient    *http.Client
	baseTransport http.RoundTripper
	logger        *slog.Logger

	// superseded names the options that WithHTTPClient would silently drop.
	superseded []string
}

// supersedable builds an option that WithHTTPClient overrides, recording its
// name so New can report the conflict instead of dropping it. Every option
// except WithEndpoint and WithHTTPClient is built this way.
func supersedable(name string, apply func(*options)) Option {
	return func(o *options) {
		apply(o)
		o.superseded = append(o.superseded, name)
	}
}

func defaultOptions() *options {
	return &options{
		endpoint:   ClientEndpoint,
		tokenURL:   TokenURL,
		userAgent:  defaultUserAgent,
		maxRetries: 3,
		timeout:    60 * time.Second,
		logger:     discardLogger,
	}
}

// WithClientCredentials authenticates using the OAuth 2.0 client-credentials
// flow against [ClientEndpoint].
func WithClientCredentials(id, secret string) Option {
	return supersedable("WithClientCredentials", func(o *options) {
		o.clientID = id
		o.clientSecret = secret
	})
}

// WithTokenSource authenticates using a caller-provided token source, such as
// one obtained from the authorization-code or PKCE flow (see [OAuthConfig]).
// Pair it with WithEndpoint([UserEndpoint]) to access private data.
func WithTokenSource(ts oauth2.TokenSource) Option {
	return supersedable("WithTokenSource", func(o *options) { o.tokenSource = ts })
}

// WithHTTPClient uses a fully preconfigured HTTP client verbatim, including its
// authentication transport. It supersedes every other auth and transport
// option, so combining it with one returns [ErrConflictingOptions] rather than
// dropping it. [WithEndpoint] still applies.
func WithHTTPClient(hc *http.Client) Option {
	return func(o *options) { o.httpClient = hc }
}

// WithEndpoint overrides the GraphQL endpoint, e.g. [UserEndpoint] or a
// region-specific host.
func WithEndpoint(url string) Option {
	return func(o *options) { o.endpoint = url }
}

// WithTokenURL overrides the OAuth 2.0 token endpoint used by
// [WithClientCredentials]. Defaults to [TokenURL].
func WithTokenURL(url string) Option {
	return supersedable("WithTokenURL", func(o *options) { o.tokenURL = url })
}

// WithScopes sets the OAuth scopes requested by the client-credentials flow.
func WithScopes(scopes ...string) Option {
	return supersedable("WithScopes", func(o *options) { o.scopes = scopes })
}

// WithUserAgent sets the User-Agent header sent with every request.
func WithUserAgent(ua string) Option {
	return supersedable("WithUserAgent", func(o *options) { o.userAgent = ua })
}

// WithMaxRetries sets how many times a retryable request (HTTP 429 or 5xx) is
// retried with exponential backoff. Zero disables retries.
func WithMaxRetries(n int) Option {
	return supersedable("WithMaxRetries", func(o *options) { o.maxRetries = n })
}

// WithTimeout sets the per-request timeout of the underlying HTTP client.
func WithTimeout(d time.Duration) Option {
	return supersedable("WithTimeout", func(o *options) { o.timeout = d })
}

// WithBaseTransport sets the http.RoundTripper beneath the retry and auth
// layers. Defaults to http.DefaultTransport.
func WithBaseTransport(rt http.RoundTripper) Option {
	return supersedable("WithBaseTransport", func(o *options) { o.baseTransport = rt })
}

// WithLogger logs retried requests at debug level to l. Request headers are
// never logged. Nothing is logged by default.
func WithLogger(l *slog.Logger) Option {
	return supersedable("WithLogger", func(o *options) {
		if l == nil {
			l = discardLogger
		}
		o.logger = l
	})
}

func (o *options) httpClientFor(ctx context.Context) (*http.Client, error) {
	if o.httpClient != nil {
		if len(o.superseded) > 0 {
			return nil, fmt.Errorf("%w: remove %s", ErrConflictingOptions, strings.Join(o.superseded, ", "))
		}
		return o.httpClient, nil
	}

	base := o.baseTransport
	if base == nil {
		base = http.DefaultTransport
	}
	base = &transport{base: base, userAgent: o.userAgent, maxRetries: o.maxRetries, logger: o.logger}

	ts := o.tokenSource
	if ts == nil {
		if o.clientID == "" || o.clientSecret == "" {
			return nil, ErrNoCredentials
		}
		cc := clientcredentials.Config{
			ClientID:     o.clientID,
			ClientSecret: o.clientSecret,
			TokenURL:     o.tokenURL,
			Scopes:       o.scopes,
			AuthStyle:    oauth2.AuthStyleInHeader,
		}
		// Token fetches would otherwise use http.DefaultClient.
		tokenCtx := context.WithValue(context.WithoutCancel(ctx), oauth2.HTTPClient,
			&http.Client{Timeout: o.timeout, Transport: base})
		ts = cc.TokenSource(tokenCtx)
	}

	return &http.Client{
		Timeout:   o.timeout,
		Transport: &oauth2.Transport{Source: ts, Base: base},
	}, nil
}
