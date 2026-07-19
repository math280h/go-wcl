package warcraftlogs_test

import (
	"context"
	"fmt"
	"log"

	warcraftlogs "github.com/math280h/go-wcl"
)

func ExampleNew() {
	ctx := context.Background()
	client, err := warcraftlogs.New(ctx,
		warcraftlogs.WithClientCredentials("client-id", "client-secret"))
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

func ExampleClient_Execute() {
	ctx := context.Background()
	client, err := warcraftlogs.New(ctx,
		warcraftlogs.WithClientCredentials("client-id", "client-secret"))
	if err != nil {
		log.Fatal(err)
	}

	var resp struct {
		WorldData struct {
			Zones []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"zones"`
		} `json:"worldData"`
	}
	if err := client.Execute(ctx, `query { worldData { zones { id name } } }`, nil, &resp); err != nil {
		log.Fatal(err)
	}
	for _, z := range resp.WorldData.Zones {
		fmt.Println(z.ID, z.Name)
	}
}

func ExampleClient_RateLimit() {
	ctx := context.Background()
	client, err := warcraftlogs.New(ctx,
		warcraftlogs.WithClientCredentials("client-id", "client-secret"))
	if err != nil {
		log.Fatal(err)
	}

	limit, err := client.RateLimit(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%.1f / %d points used\n", limit.PointsSpentThisHour, limit.LimitPerHour)
}

func ExampleOAuthConfig() {
	ctx := context.Background()
	cfg := warcraftlogs.OAuthConfig("client-id", "client-secret", "https://example.com/callback")

	url := cfg.AuthCodeURL("state-token")
	fmt.Println("visit:", url)

	tok, err := cfg.Exchange(ctx, "code-from-callback")
	if err != nil {
		log.Fatal(err)
	}

	client, err := warcraftlogs.New(ctx,
		warcraftlogs.WithTokenSource(cfg.TokenSource(ctx, tok)),
		warcraftlogs.WithEndpoint(warcraftlogs.UserEndpoint))
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}
