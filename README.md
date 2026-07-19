# go-wcl

[![Go Reference](https://pkg.go.dev/badge/github.com/math280h/go-wcl.svg)](https://pkg.go.dev/github.com/math280h/go-wcl)

`go-wcl` is a Go client for the [Warcraft Logs v2 GraphQL API](https://www.warcraftlogs.com/api/docs).

It handles OAuth 2.0 authentication, request retries, and rate-limit inspection.
Typed methods are generated from the API schema, and `Client.Execute` runs
arbitrary queries for anything the typed methods do not cover.

## Features

- OAuth 2.0 client-credentials, authorization-code, and PKCE flows.
- Type-safe methods and models generated from the live schema with [genqlient](https://github.com/Khan/genqlient).
- `Execute` runs any raw GraphQL query the generated methods don't cover.
- Automatic retries with exponential backoff, honoring `Retry-After`.
- Rate-limit inspection and typed error helpers.
- Minimal runtime dependencies.

## Requirements

- Go 1.25 or later.
- A Warcraft Logs API client. Create one on the [client management
  page](https://www.warcraftlogs.com/api/clients/) to get a client ID and secret.

## Installation

```sh
go get github.com/math280h/go-wcl
```

## Usage

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	warcraftlogs "github.com/math280h/go-wcl"
)

func main() {
	ctx := context.Background()

	client, err := warcraftlogs.New(ctx,
		warcraftlogs.WithClientCredentials("client-id", "client-secret"))
	if err != nil {
		log.Fatal(err)
	}

	char, err := client.CharacterByName(ctx, "Asmongold", "area-52", "us")
	if errors.Is(err, warcraftlogs.ErrNotFound) {
		log.Fatal("character not found")
	} else if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s - %s (%s)\n", char.Name, char.Server.Name, char.Server.Region.Name)
}
```

Lookups that resolve to a missing entity return
[`ErrNotFound`](https://pkg.go.dev/github.com/math280h/go-wcl#ErrNotFound), as
shown above. An unknown report code is rejected by the API itself and arrives
as a GraphQL error instead. See
[`GraphQLErrors`](https://pkg.go.dev/github.com/math280h/go-wcl#GraphQLErrors).

### Authentication

Public data is accessed with the client-credentials flow against
[`ClientEndpoint`](https://pkg.go.dev/github.com/math280h/go-wcl#pkg-constants):

```go
client, err := warcraftlogs.New(ctx,
	warcraftlogs.WithClientCredentials(id, secret))
```

Private data (a user's own reports, `CurrentUser`) requires the
authorization-code or PKCE flow against `UserEndpoint`. Use `OAuthConfig` to
build the authorization URL and exchange the returned code, then pass the token
to the client:

```go
cfg := warcraftlogs.OAuthConfig(id, secret, "https://example.com/callback")

// 1. Redirect the user to the authorization URL.
url := cfg.AuthCodeURL("state-token")

// 2. On the callback, exchange the code for a token.
tok, err := cfg.Exchange(ctx, codeFromCallback)
if err != nil {
	log.Fatal(err)
}

// 3. Build a client for the private API.
client, err := warcraftlogs.New(ctx,
	warcraftlogs.WithTokenSource(cfg.TokenSource(ctx, tok)),
	warcraftlogs.WithEndpoint(warcraftlogs.UserEndpoint))
```

For PKCE (clients that cannot hold a secret), pass an empty secret and use the
verifier options from `golang.org/x/oauth2`:

```go
cfg := warcraftlogs.OAuthConfig(id, "", "https://example.com/callback")
verifier := oauth2.GenerateVerifier()
url := cfg.AuthCodeURL("state-token", oauth2.S256ChallengeOption(verifier))
tok, err := cfg.Exchange(ctx, codeFromCallback, oauth2.VerifierOption(verifier))
```

[`examples/userauth`](examples/userauth) is a runnable version of this flow: a
local redirect server, `state` validation, the PKCE exchange, and a
`CurrentUser` call against `UserEndpoint`.

```sh
go run ./examples/userauth
go run ./examples/userauth -redirect http://localhost:9000/callback
```

### Typed queries

Methods cover characters, guilds, reports, world data, game data, and users.

```go
// Reports and fights.
report, err := client.Report(ctx, "aBcDeFgHiJkLmN01", false)

fights, err := client.ReportFights(ctx, warcraftlogs.ReportFightsParams{
	Code:     "aBcDeFgHiJkLmN01",
	KillType: warcraftlogs.KillTypeKills,
})
for _, f := range fights {
	fmt.Printf("%s (kill=%t)\n", f.Name, f.Kill)
}

// World data.
zones, err := client.Zones(ctx, 0) // 0 = all expansions
encounter, err := client.Encounter(ctx, 3009)
```

### Analysis endpoints

Rankings, tables, graphs, events, and player details are returned as
`json.RawMessage`, matching the API's `JSON` type. Decode them into your own
structs.

```go
data, err := client.CharacterZoneRankings(ctx, warcraftlogs.ZoneRankingsParams{
	Character: warcraftlogs.CharacterRef{Name: "Asmongold", ServerSlug: "area-52", ServerRegion: "us"},
	Metric:    warcraftlogs.CharacterPageRankingMetricTypeDps,
})

table, err := client.ReportTable(ctx, warcraftlogs.TableDataTypeDamagedone,
	warcraftlogs.ReportAnalysisParams{Code: "aBcDeFgHiJkLmN01"})
```

Events are paginated. `NextPageTimestamp` is zero on the last page; otherwise
pass it as the next `StartTime`:

```go
params := warcraftlogs.ReportEventsParams{Code: "aBcDeFgHiJkLmN01", FightIDs: []int{12}}
for {
	page, err := client.ReportEvents(ctx, warcraftlogs.EventDataTypeDeaths, params)
	if err != nil {
		log.Fatal(err)
	}
	// ... handle page.Data ...
	if page.NextPageTimestamp == 0 {
		break
	}
	params.StartTime = page.NextPageTimestamp
}
```

Events require either `FightIDs` or an explicit `StartTime`/`EndTime` range.

[`examples/analysis`](examples/analysis) is a runnable walkthrough of a real
log: report metadata, a per-boss pull summary, the damage breakdown of a kill,
and every death joined against report master data to resolve actor and ability
names.

```sh
go run ./examples/analysis
go run ./examples/analysis -report aBcDeFgHiJkLmN01
```

### Raw queries

`Execute` runs any query and decodes the `data` field into a pointer. Use it for
operations not covered by the typed methods.

```go
var resp struct {
	WorldData struct {
		Regions []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"regions"`
	} `json:"worldData"`
}
err := client.Execute(ctx, `query { worldData { regions { id name } } }`, nil, &resp)
```

### Rate limiting

The API uses an hourly point budget. Inspect it at any time:

```go
limit, err := client.RateLimit(ctx)
fmt.Printf("%.1f / %d points used, resets in %ds\n",
	limit.PointsSpentThisHour, limit.LimitPerHour, limit.PointsResetIn)
```

Requests that return HTTP 429 or 5xx are retried automatically with backoff
(configurable via `WithMaxRetries`).

### Errors

Helpers classify errors returned by any method:

```go
if _, err := client.Report(ctx, code, false); err != nil {
	if warcraftlogs.IsRateLimited(err) {
		// back off
	}
	for _, ge := range warcraftlogs.GraphQLErrors(err) {
		// e.g. "graphql: reportData.report: This report does not exist."
		log.Printf("graphql: %s: %s", ge.Path, ge.Message)
	}
	if status, ok := warcraftlogs.HTTPStatus(err); ok {
		log.Printf("http status: %d", status)
	}
}
```

### Client options

`New` accepts functional options:

| Option | Purpose |
| --- | --- |
| `WithClientCredentials(id, secret)` | Client-credentials authentication. |
| `WithTokenSource(ts)` | Authenticate with a caller-provided `oauth2.TokenSource`. |
| `WithHTTPClient(hc)` | Use a preconfigured `*http.Client` verbatim. |
| `WithEndpoint(url)` | Override the GraphQL endpoint (e.g. `UserEndpoint`). |
| `WithScopes(scopes...)` | Scopes for the client-credentials flow. |
| `WithUserAgent(ua)` | Set the `User-Agent` header. |
| `WithMaxRetries(n)` | Retry attempts for 429/5xx responses (default 3). |
| `WithTimeout(d)` | Per-request timeout (default 60s). |
| `WithBaseTransport(rt)` | `http.RoundTripper` beneath the retry and auth layers. |
| `WithLogger(l)` | Log retried requests to a `*slog.Logger`. Silent by default. |

## Development

The typed layer is generated from a committed copy of the schema
(`schema/schema.graphql`) and the operations under `operations/`.

Copy `.env.example` to `.env` and fill in your credentials:

```sh
task              # list tasks
task check        # fmt check, vet, build, and unit tests
task regenerate   # refresh the schema, then regenerate the typed client
task test:integration  # run tests against the live API
```

Each task maps to plain Go commands if you prefer to run them directly:

```sh
go generate ./...                 # regenerate from operations + schema
go -C tools run ./introspect      # refresh schema/schema.graphql
go test ./...                     # unit tests
go test -tags integration ./...   # live API tests (skipped without credentials)
```

## Disclaimer

This project is not affiliated with or endorsed by Warcraft Logs or Blizzard
Entertainment. It's an unofficial client, maintained independently. All
trademarks belong to their respective owners.

You're bound by the [Warcraft Logs terms of service](https://forums.combatlogforums.com/tos)
when using their API through this library.

## License

See [`LICENSE`](LICENSE).
