// Package warcraftlogs is a client for the Warcraft Logs v2 GraphQL API.
//
// It handles OAuth 2.0 authentication (client credentials, authorization code,
// and PKCE), request retries, and rate-limit inspection. Typed operations are
// generated from the API schema; [Client.Execute] runs arbitrary queries for
// anything the typed layer does not cover.
//
//	client, err := warcraftlogs.New(ctx,
//		warcraftlogs.WithClientCredentials(id, secret))
//	if err != nil {
//		return err
//	}
//
//	limit, err := client.RateLimit(ctx)
package warcraftlogs
