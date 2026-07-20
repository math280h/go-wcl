package warcraftlogs

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestWithTokenURLDrivesTheCredentialsFlow(t *testing.T) {
	var tokenCalls int
	var gotUserAgent string

	tokens := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenCalls++
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "test-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokens.Close()

	var gotAuth string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"rateLimitData":{"limitPerHour":9000,"pointsSpentThisHour":1.5,"pointsResetIn":60}}}`))
	}))
	defer api.Close()

	client, err := New(context.Background(),
		WithClientCredentials("id", "secret"),
		WithTokenURL(tokens.URL),
		WithEndpoint(api.URL),
		WithUserAgent("test-agent"))
	if err != nil {
		t.Fatal(err)
	}

	limit, err := client.RateLimit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if limit.LimitPerHour != 9000 {
		t.Errorf("LimitPerHour = %d, want 9000", limit.LimitPerHour)
	}
	if tokenCalls != 1 {
		t.Errorf("token endpoint called %d times, want 1", tokenCalls)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	// Token fetches go through the same transport as API calls.
	if gotUserAgent != "test-agent" {
		t.Errorf("token request User-Agent = %q, want test-agent", gotUserAgent)
	}
}

func TestTokenURLDefaultsToTheConstant(t *testing.T) {
	if got := defaultOptions().tokenURL; got != TokenURL {
		t.Errorf("default tokenURL = %q, want %q", got, TokenURL)
	}
}

func TestWithHTTPClientRejectsSupersededOptions(t *testing.T) {
	hc := &http.Client{}
	for _, tc := range []struct {
		name string
		opt  Option
	}{
		{"WithClientCredentials", WithClientCredentials("id", "secret")},
		{"WithTokenSource", WithTokenSource(oauth2.StaticTokenSource(&oauth2.Token{}))},
		{"WithTokenURL", WithTokenURL("https://example.com/token")},
		{"WithScopes", WithScopes("view-user-profile")},
		{"WithUserAgent", WithUserAgent("test")},
		{"WithMaxRetries", WithMaxRetries(5)},
		{"WithTimeout", WithTimeout(time.Second)},
		{"WithBaseTransport", WithBaseTransport(http.DefaultTransport)},
		{"WithLogger", WithLogger(slog.Default())},
	} {
		_, err := New(context.Background(), WithHTTPClient(hc), tc.opt)
		if !errors.Is(err, ErrConflictingOptions) {
			t.Errorf("New(WithHTTPClient, %s) err = %v, want ErrConflictingOptions", tc.name, err)
			continue
		}
		if !strings.Contains(err.Error(), tc.name) {
			t.Errorf("error does not name %s: %v", tc.name, err)
		}
	}
}

// WithEndpoint is read outside the transport, so it composes.
func TestWithHTTPClientAllowsEndpoint(t *testing.T) {
	client, err := New(context.Background(),
		WithHTTPClient(&http.Client{}), WithEndpoint(UserEndpoint))
	if err != nil {
		t.Fatal(err)
	}
	if client.Endpoint() != UserEndpoint {
		t.Errorf("Endpoint = %q, want %q", client.Endpoint(), UserEndpoint)
	}
}

func TestNewWithoutCredentials(t *testing.T) {
	_, err := New(context.Background(), WithEndpoint("https://example.com"))
	if !errors.Is(err, ErrNoCredentials) {
		t.Errorf("err = %v, want ErrNoCredentials", err)
	}
}
