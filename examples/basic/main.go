package main

import (
	"context"
	"fmt"
	"log"
	"os"

	warcraftlogs "github.com/math280h/go-wcl"
)

func main() {
	ctx := context.Background()

	id, secret := os.Getenv("WCL_CLIENT_ID"), os.Getenv("WCL_CLIENT_SECRET")
	if id == "" || secret == "" {
		log.Fatal("set WCL_CLIENT_ID and WCL_CLIENT_SECRET")
	}

	client, err := warcraftlogs.New(ctx, warcraftlogs.WithClientCredentials(id, secret))
	if err != nil {
		log.Fatal(err)
	}

	limit, err := client.RateLimit(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("rate limit: %.1f / %d points used, resets in %ds\n",
		limit.PointsSpentThisHour, limit.LimitPerHour, limit.PointsResetIn)
}
