package main

import (
	"context"
	"errors"
	"fmt"

	warcraftlogs "github.com/math280h/go-wcl"
)

// demoErrors shows the error categories the package distinguishes.
func demoErrors(ctx context.Context, client *warcraftlogs.Client) {
	fmt.Printf("\n== error handling ==\n")

	if _, err := client.Character(ctx, 999999999); errors.Is(err, warcraftlogs.ErrNotFound) {
		fmt.Println("missing character:  ErrNotFound")
	}

	_, err := client.ReportFights(ctx, warcraftlogs.ReportFightsParams{Code: "zzzzzzzzzzzzzzzz"})
	for _, ge := range warcraftlogs.GraphQLErrors(err) {
		fmt.Printf("bad report code:    %s: %s\n", ge.Path, ge.Message)
	}
	if status, ok := warcraftlogs.HTTPStatus(err); ok {
		fmt.Printf("http status:        %d (rate limited: %v)\n", status, warcraftlogs.IsRateLimited(err))
	}
}
