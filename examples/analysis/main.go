// Command analysis walks a single report end to end: metadata, a fight
// summary, the damage breakdown of the last boss kill, and the deaths of the
// costliest pull. It shows how to decode the json.RawMessage payloads the
// analysis endpoints return and how to join them against report master data.
//
//	export WCL_CLIENT_ID=... WCL_CLIENT_SECRET=...
//	go run ./examples/analysis
//	go run ./examples/analysis -report aBcDeFgHiJkLmN01
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	warcraftlogs "github.com/math280h/go-wcl"
)

// A public log used when -report is not supplied.
const defaultReport = "9jhrD3RPxzCV2gZq"

func main() {
	code := flag.String("report", defaultReport, "report code to analyze")
	flag.Parse()

	if err := run(context.Background(), *code); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, code string) error {
	id, secret := os.Getenv("WCL_CLIENT_ID"), os.Getenv("WCL_CLIENT_SECRET")
	if id == "" || secret == "" {
		return errors.New("set WCL_CLIENT_ID and WCL_CLIENT_SECRET")
	}

	client, err := warcraftlogs.New(ctx,
		warcraftlogs.WithClientCredentials(id, secret),
		warcraftlogs.WithUserAgent("go-wcl-example/1.0"),
		warcraftlogs.WithTimeout(30*time.Second),
		warcraftlogs.WithMaxRetries(3))
	if err != nil {
		return err
	}

	before, err := client.RateLimit(ctx)
	if err != nil {
		return err
	}

	// One request for the header, the fights and the encounter phases.
	report, err := client.ReportWithFights(ctx, warcraftlogs.ReportWithFightsParams{Code: code})
	if err != nil {
		return err
	}

	describeReport(code, report)
	if lastKill := summarizeFights(report); lastKill != nil {
		if err := topDamage(ctx, client, code, *lastKill); err != nil {
			return err
		}
	}
	if err := deaths(ctx, client, code, report.Fights); err != nil {
		return err
	}

	demoErrors(ctx, client)

	after, err := client.RateLimit(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\n== rate limit ==\nspent %.2f points on this run, %.0f of %d used this hour\n",
		after.PointsSpentThisHour-before.PointsSpentThisHour,
		after.PointsSpentThisHour, after.LimitPerHour)
	return nil
}
