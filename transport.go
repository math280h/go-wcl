package warcraftlogs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	retryBaseDelay = 500 * time.Millisecond
	retryMaxDelay  = 30 * time.Second

	drainLimit = 64 << 10
	peekLimit  = 4 << 10
)

// transport adds default headers and retries retryable responses with
// exponential backoff, honoring the Retry-After header when present. Responses
// served by the CDN rather than the API fail with a [*CDNError].
type transport struct {
	base       http.RoundTripper
	userAgent  string
	maxRetries int
	logger     *slog.Logger
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	if r.Header.Get("Accept") == "" {
		r.Header.Set("Accept", "application/json")
	}
	if t.userAgent != "" && r.Header.Get("User-Agent") == "" {
		r.Header.Set("User-Agent", t.userAgent)
	}

	logger := t.logger
	if logger == nil {
		logger = discardLogger
	}

	ctx := r.Context()
	for attempt := 0; ; attempt++ {
		if attempt > 0 && r.GetBody != nil {
			body, err := r.GetBody()
			if err != nil {
				return nil, err
			}
			r.Body = body
		}

		resp, err := t.base.RoundTrip(r)

		var wait time.Duration
		switch {
		case err != nil:
			if attempt >= t.maxRetries || !retryableErr(err) {
				return nil, err
			}
			wait = backoff(attempt)
			logger.DebugContext(ctx, "retrying request",
				"method", r.Method, "url", r.URL.String(),
				"attempt", attempt+1, "delay", wait, "error", err)
		case retryableStatus(resp.StatusCode) && !cdnBlocked(resp):
			if attempt >= t.maxRetries {
				return finish(r, resp)
			}
			wait = retryAfter(resp, attempt)
			drain(resp)
			logger.DebugContext(ctx, "retrying request",
				"method", r.Method, "url", r.URL.String(),
				"attempt", attempt+1, "delay", wait, "status", resp.StatusCode)
		default:
			return finish(r, resp)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

func retryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	// Warcraft Logs sits behind Cloudflare, which serves 520-527 from its own
	// edge when the origin misbehaves. They are transient and worth retrying.
	return code >= 520 && code <= 527
}

func retryableErr(err error) bool {
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func backoff(attempt int) time.Duration {
	d := retryBaseDelay << attempt
	if d <= 0 || d > retryMaxDelay {
		d = retryMaxDelay
	}
	half := d / 2
	return half + time.Duration(rand.Int64N(int64(half)+1))
}

func retryAfter(resp *http.Response, attempt int) time.Duration {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return backoff(attempt)
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(v); err == nil {
		if d := time.Until(when); d > 0 {
			return d
		}
	}
	return backoff(attempt)
}

// finish reports an edge-served response as a [*CDNError], so callers see that
// the CDN blocked them rather than a JSON syntax error from the HTML body.
func finish(req *http.Request, resp *http.Response) (*http.Response, error) {
	if !isCDNInterstitial(resp) {
		return resp, nil
	}
	err := &CDNError{
		StatusCode: resp.StatusCode,
		URL:        req.URL.String(),
		Title:      htmlTitle(peek(resp)),
	}
	drain(resp)
	return nil, err
}

// isCDNInterstitial reports whether resp is an edge-served HTML page. The API
// only ever answers with JSON, so HTML means the request never reached it.
func isCDNInterstitial(resp *http.Response) bool {
	if cdnBlocked(resp) {
		return true
	}
	mediaType, _, _ := strings.Cut(resp.Header.Get("Content-Type"), ";")
	return strings.EqualFold(strings.TrimSpace(mediaType), "text/html")
}

// cdnBlocked reports whether Cloudflare says it acted on the request itself.
// Unlike a bare HTML body, this cannot be a transient origin error, so it is
// not worth retrying.
func cdnBlocked(resp *http.Response) bool {
	return resp.Header.Get("cf-mitigated") != ""
}

// peek reads the head of a body that is about to be discarded.
func peek(resp *http.Response) string {
	if resp.Body == nil {
		return ""
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, peekLimit))
	return string(b)
}

// htmlTitle returns the first <title> element in body, collapsed to one line.
func htmlTitle(body string) string {
	lower := strings.ToLower(body)
	open := strings.Index(lower, "<title")
	if open < 0 {
		return ""
	}
	start := strings.Index(lower[open:], ">")
	if start < 0 {
		return ""
	}
	start += open + 1
	end := strings.Index(lower[start:], "</title>")
	if end < 0 {
		return ""
	}
	return strings.Join(strings.Fields(body[start:start+end]), " ")
}

// drain reads up to drainLimit of a discarded body so the connection can be
// reused by the retry.
func drain(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, drainLimit))
		_ = resp.Body.Close()
	}
}
