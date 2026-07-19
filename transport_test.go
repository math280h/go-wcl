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
		{519, false},
		{520, true}, // Cloudflare unknown error
		{522, true}, // Cloudflare connection timed out
		{527, true},
		{528, false},
	} {
		if got := retryableStatus(tc.code); got != tc.want {
			t.Errorf("retryableStatus(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

// htmlResp builds a Cloudflare-style interstitial.
func htmlResp(code int, body string) *http.Response {
	r := resp(code)
	r.Header.Set("Content-Type", "text/html; charset=UTF-8")
	r.Body = io.NopCloser(strings.NewReader(body))
	return r
}

func TestRoundTripReportsCDNChallenge(t *testing.T) {
	base := &stubRoundTripper{responses: []*http.Response{
		htmlResp(http.StatusForbidden, "<html><head>\n<title>Just a moment...</title>\n</head></html>"),
	}}
	tr := &transport{base: base, maxRetries: 3}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/api", nil)
	got, err := tr.RoundTrip(req)
	if got != nil {
		t.Errorf("response = %v, want nil", got)
	}
	var ce *CDNError
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v, want *CDNError", err)
	}
	if ce.StatusCode != http.StatusForbidden || ce.Title != "Just a moment..." {
		t.Errorf("CDNError = %+v", ce)
	}
	if base.calls != 1 {
		t.Errorf("calls = %d, want 1 (403 is not retryable)", base.calls)
	}
}

// A challenge served as a retryable status is still classified once retries run out.
func TestRoundTripReportsCDNChallengeAfterRetries(t *testing.T) {
	base := &stubRoundTripper{responses: []*http.Response{
		htmlResp(http.StatusServiceUnavailable, "<html><title>Attention Required!</title></html>"),
		htmlResp(http.StatusServiceUnavailable, "<html><title>Attention Required!</title></html>"),
	}}
	tr := &transport{base: base, maxRetries: 1}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/api", nil)
	if _, err := tr.RoundTrip(req); !IsBlocked(err) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if base.calls != 2 {
		t.Errorf("calls = %d, want 2", base.calls)
	}
}

func TestRoundTripPassesJSONThrough(t *testing.T) {
	ok := resp(http.StatusOK)
	ok.Header.Set("Content-Type", "application/json")
	base := &stubRoundTripper{responses: []*http.Response{ok}}
	tr := &transport{base: base}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/api", nil)
	got, err := tr.RoundTrip(req)
	if err != nil || got != ok {
		t.Fatalf("got %v, %v, want the response unchanged", got, err)
	}
}

// A GraphQL error body is JSON with a non-2xx status; it must not be mistaken
// for a CDN block.
func TestRoundTripDoesNotBlockOnJSONErrorBody(t *testing.T) {
	bad := &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"errors":[{"message":"bad"}]}`)),
	}
	tr := &transport{base: &stubRoundTripper{responses: []*http.Response{bad}}}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/api", nil)
	got, err := tr.RoundTrip(req)
	if err != nil || got != bad {
		t.Fatalf("got %v, %v, want the response unchanged", got, err)
	}
}

func TestRoundTripDetectsCFMitigatedHeader(t *testing.T) {
	blocked := resp(http.StatusOK)
	blocked.Header.Set("cf-mitigated", "challenge")
	tr := &transport{base: &stubRoundTripper{responses: []*http.Response{blocked}}}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/api", nil)
	if _, err := tr.RoundTrip(req); !IsBlocked(err) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
}

// cf-mitigated means Cloudflare acted on the request, so retrying an otherwise
// retryable status cannot help.
func TestRoundTripDoesNotRetryCFMitigated(t *testing.T) {
	blocked := resp(http.StatusServiceUnavailable)
	blocked.Header.Set("cf-mitigated", "challenge")
	base := &stubRoundTripper{responses: []*http.Response{blocked}}
	tr := &transport{base: base, maxRetries: 3}

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://example.com/api", nil)
	if _, err := tr.RoundTrip(req); !IsBlocked(err) {
		t.Fatalf("err = %v, want ErrBlocked", err)
	}
	if base.calls != 1 {
		t.Errorf("calls = %d, want 1", base.calls)
	}
}

func TestHTMLTitle(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"<html><title>Just a moment...</title></html>", "Just a moment..."},
		{"<title class=\"x\">  Attention\n  Required  </title>", "Attention Required"},
		{"<html><head></head></html>", ""},
		{"<title>unterminated", ""},
		{"", ""},
	} {
		if got := htmlTitle(tc.body); got != tc.want {
			t.Errorf("htmlTitle(%q) = %q, want %q", tc.body, got, tc.want)
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
