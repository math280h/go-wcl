package warcraftlogs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

const (
	retryBaseDelay = 500 * time.Millisecond
	retryMaxDelay  = 30 * time.Second

	drainLimit = 64 << 10
)

// transport adds default headers and retries retryable responses with
// exponential backoff, honoring the Retry-After header when present.
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
		case retryableStatus(resp.StatusCode):
			if attempt >= t.maxRetries {
				return resp, nil
			}
			wait = retryAfter(resp, attempt)
			drain(resp)
			logger.DebugContext(ctx, "retrying request",
				"method", r.Method, "url", r.URL.String(),
				"attempt", attempt+1, "delay", wait, "status", resp.StatusCode)
		default:
			return resp, nil
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
	default:
		return false
	}
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

// drain reads up to drainLimit of a discarded body so the connection can be
// reused by the retry.
func drain(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, drainLimit))
		_ = resp.Body.Close()
	}
}
