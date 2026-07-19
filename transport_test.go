package warcraftlogs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"
)

type stubRoundTripper struct {
	responses []*http.Response
	errs      []error
	calls     int
}

func (s *stubRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	i := s.calls
	s.calls++
	if i < len(s.errs) && s.errs[i] != nil {
		return nil, s.errs[i]
	}
	return s.responses[i], nil
}

// resp builds a response with Retry-After: 0 so retries do not sleep.
func resp(code int) *http.Response {
	return &http.Response{
		StatusCode: code,
		Header:     http.Header{"Retry-After": []string{"0"}},
		Body:       io.NopCloser(strings.NewReader("body")),
	}
}

func TestRoundTripRetriesUntilSuccess(t *testing.T) {
	base := &stubRoundTripper{responses: []*http.Response{
		resp(http.StatusTooManyRequests),
		resp(http.StatusBadGateway),
		resp(http.StatusOK),
	}}
	tr := &transport{base: base, maxRetries: 3, userAgent: "test-agent"}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	got, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if got.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", got.StatusCode)
	}
	if base.calls != 3 {
		t.Errorf("calls = %d, want 3", base.calls)
	}
}

func TestRoundTripGivesUpAfterMaxRetries(t *testing.T) {
	base := &stubRoundTripper{responses: []*http.Response{
		resp(http.StatusServiceUnavailable),
		resp(http.StatusServiceUnavailable),
	}}
	tr := &transport{base: base, maxRetries: 1}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	got, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if got.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", got.StatusCode)
	}
	if base.calls != 2 {
		t.Errorf("calls = %d, want 2", base.calls)
	}
}

func TestRoundTripSetsDefaultHeaders(t *testing.T) {
	var seen *http.Request
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen = r
		return resp(http.StatusOK), nil
	})
	tr := &transport{base: base, userAgent: "test-agent"}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
	if got := seen.Header.Get("User-Agent"); got != "test-agent" {
		t.Errorf("User-Agent = %q", got)
	}
	if got := seen.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
	if req.Header.Get("User-Agent") != "" {
		t.Error("original request was mutated")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRoundTripDoesNotRetryCanceledContext(t *testing.T) {
	base := &stubRoundTripper{
		responses: []*http.Response{nil},
		errs:      []error{context.Canceled},
	}
	tr := &transport{base: base, maxRetries: 3}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if _, err := tr.RoundTrip(req); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if base.calls != 1 {
		t.Errorf("calls = %d, want 1", base.calls)
	}
}

func TestRoundTripLogsRetries(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	base := &stubRoundTripper{responses: []*http.Response{
		resp(http.StatusTooManyRequests),
		resp(http.StatusOK),
	}}
	tr := &transport{base: base, maxRetries: 2, logger: logger}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/api", nil)
	req.Header.Set("Authorization", "Bearer super-secret-token")
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, want := range []string{"retrying request", "POST", "https://example.com/api", `"status":429`, `"attempt":1`} {
		if !strings.Contains(out, want) {
			t.Errorf("log missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "super-secret-token") || strings.Contains(out, "Authorization") {
		t.Errorf("log leaked request headers:\n%s", out)
	}
}

func TestRoundTripWithoutLoggerDoesNotPanic(t *testing.T) {
	base := &stubRoundTripper{responses: []*http.Response{
		resp(http.StatusBadGateway),
		resp(http.StatusOK),
	}}
	tr := &transport{base: base, maxRetries: 1}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.com", nil)
	if _, err := tr.RoundTrip(req); err != nil {
		t.Fatal(err)
	}
}

func TestRetryableStatus(t *testing.T) {
	for _, tc := range []struct {
		code int
		want bool
	}{
		{http.StatusOK, false},
		{http.StatusNotFound, false},
		{http.StatusTooManyRequests, true},
		{http.StatusInternalServerError, true},
		{http.StatusBadGateway, true},
		{http.StatusServiceUnavailable, true},
		{http.StatusGatewayTimeout, true},
	} {
		if got := retryableStatus(tc.code); got != tc.want {
			t.Errorf("retryableStatus(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	for attempt := range 4 {
		d := backoff(attempt)
		max := retryBaseDelay << attempt
		if max > retryMaxDelay {
			max = retryMaxDelay
		}
		if d < max/2 || d > max {
			t.Errorf("backoff(%d) = %v, want within [%v, %v]", attempt, d, max/2, max)
		}
	}
	if d := backoff(60); d > retryMaxDelay {
		t.Errorf("backoff(60) = %v, exceeds cap %v", d, retryMaxDelay)
	}
}

func TestRetryAfter(t *testing.T) {
	seconds := &http.Response{Header: http.Header{"Retry-After": []string{"12"}}}
	if got := retryAfter(seconds, 0); got != 12*time.Second {
		t.Errorf("seconds form = %v, want 12s", got)
	}

	future := time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)
	date := &http.Response{Header: http.Header{"Retry-After": []string{future}}}
	if got := retryAfter(date, 0); got <= 0 || got > 31*time.Second {
		t.Errorf("date form = %v, want ~30s", got)
	}

	// A past date and a missing header both fall back to backoff.
	past := time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)
	for _, r := range []*http.Response{
		{Header: http.Header{"Retry-After": []string{past}}},
		{Header: http.Header{}},
	} {
		if got := retryAfter(r, 0); got < retryBaseDelay/2 || got > retryBaseDelay {
			t.Errorf("fallback = %v, want a backoff delay", got)
		}
	}
}
