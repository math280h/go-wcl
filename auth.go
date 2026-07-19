package warcraftlogs

import "golang.org/x/oauth2"

// OAuthConfig returns an oauth2.Config wired to the Warcraft Logs OAuth
// endpoints for the authorization-code and PKCE flows, used to access private
// data via [UserEndpoint].
//
// Authorization-code:
//
//	cfg := warcraftlogs.OAuthConfig(id, secret, redirect)
//	url := cfg.AuthCodeURL(state)
//	// redirect the user, then on the callback:
//	tok, err := cfg.Exchange(ctx, code)
//
// PKCE (no client secret):
//
//	cfg := warcraftlogs.OAuthConfig(id, "", redirect)
//	verifier := oauth2.GenerateVerifier()
//	url := cfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))
//	tok, err := cfg.Exchange(ctx, code, oauth2.VerifierOption(verifier))
//
// In both cases build the client with the resulting token:
//
//	client, err := warcraftlogs.New(ctx,
//		warcraftlogs.WithTokenSource(cfg.TokenSource(ctx, tok)),
//		warcraftlogs.WithEndpoint(warcraftlogs.UserEndpoint))
func OAuthConfig(clientID, clientSecret, redirectURL string, scopes ...string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:   AuthorizeURL,
			TokenURL:  TokenURL,
			AuthStyle: oauth2.AuthStyleInHeader,
		},
	}
}
