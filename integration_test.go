//go:build integration

package warcraftlogs_test

import (
	"context"
	"errors"
	"os"
	"testing"

	warcraftlogs "github.com/math280h/go-wcl"
)

func newTestClient(t *testing.T) *warcraftlogs.Client {
	t.Helper()
	id, secret := os.Getenv("WCL_CLIENT_ID"), os.Getenv("WCL_CLIENT_SECRET")
	if id == "" || secret == "" {
		t.Skip("WCL_CLIENT_ID and WCL_CLIENT_SECRET not set")
	}
	client, err := warcraftlogs.New(context.Background(), warcraftlogs.WithClientCredentials(id, secret))
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestRateLimit(t *testing.T) {
	client := newTestClient(t)
	limit, err := client.RateLimit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if limit.LimitPerHour <= 0 {
		t.Fatalf("expected positive LimitPerHour, got %d", limit.LimitPerHour)
	}
	t.Logf("%.1f / %d points used, resets in %ds", limit.PointsSpentThisHour, limit.LimitPerHour, limit.PointsResetIn)
}

func TestExpansionsAndZones(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()

	expansions, err := client.Expansions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(expansions) == 0 {
		t.Fatal("expected at least one expansion")
	}
	t.Logf("latest expansion: %d %q", expansions[len(expansions)-1].Id, expansions[len(expansions)-1].Name)

	zones, err := client.Zones(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(zones) == 0 {
		t.Fatal("expected at least one zone")
	}
}

func TestClasses(t *testing.T) {
	client := newTestClient(t)
	classes, err := client.Classes(context.Background(), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) == 0 {
		t.Fatal("expected at least one class")
	}
}

func TestNotFound(t *testing.T) {
	client := newTestClient(t)
	char, err := client.Character(context.Background(), 999999999)
	if !errors.Is(err, warcraftlogs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if char != nil {
		t.Fatalf("expected nil character, got %+v", char)
	}
}

func TestCharacterRankingsNotFound(t *testing.T) {
	client := newTestClient(t)
	_, err := client.CharacterZoneRankings(context.Background(), warcraftlogs.ZoneRankingsParams{
		Character: warcraftlogs.CharacterRef{Name: "Nonexistentxyzzy", ServerSlug: "area-52", ServerRegion: "us"},
	})
	if !errors.Is(err, warcraftlogs.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestReportNotFound(t *testing.T) {
	client := newTestClient(t)
	_, err := client.ReportWithFights(context.Background(), warcraftlogs.ReportWithFightsParams{Code: "zzzzzzzzzzzzzzzz"})
	if err == nil {
		t.Fatal("expected an error for a nonexistent report code")
	}
	gqlErrs := warcraftlogs.GraphQLErrors(err)
	if len(gqlErrs) == 0 {
		t.Fatalf("expected a GraphQL error, got %v", err)
	}
	if gqlErrs[0].Path != "reportData.report" {
		t.Errorf("expected path %q, got %q", "reportData.report", gqlErrs[0].Path)
	}
	t.Logf("message=%q path=%q locations=%+v", gqlErrs[0].Message, gqlErrs[0].Path, gqlErrs[0].Locations)
}

func TestExecuteRaw(t *testing.T) {
	client := newTestClient(t)
	var resp struct {
		WorldData struct {
			Regions []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"regions"`
		} `json:"worldData"`
	}
	if err := client.Execute(context.Background(), `query { worldData { regions { id name } } }`, nil, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.WorldData.Regions) == 0 {
		t.Fatal("expected at least one region")
	}
}
