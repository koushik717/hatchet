package retry

import (
	"context"
	"io"
	"net/http"
	"time"
)

// HTTPDoer performs HTTP requests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// RestDoer retries eligible bodyless GET and HEAD requests.
type RestDoer struct {
	inner              HTTPDoer
	headerTimeoutInner HTTPDoer
	clock              clock
}

type RestDoerOption func(*RestDoer)

// WithRestClock injects deterministic time, sleep, and jitter hooks for tests.
func WithRestClock(now func() time.Time, sleep func(context.Context, time.Duration) error, jitter func(int64) int64) RestDoerOption {
	return func(d *RestDoer) {
		if now != nil {
			d.clock.now = now
		}
		if sleep != nil {
			d.clock.sleep = sleep
		}
		if jitter != nil {
			d.clock.jitter = jitter
		}
	}
}

// WithHeaderTimeout overrides the per-attempt response header timeout for tests.
func WithHeaderTimeout(timeout time.Duration) RestDoerOption {
	return func(d *RestDoer) {
		d.headerTimeoutInner = withResponseHeaderTimeout(d.inner, timeout)
	}
}

// NewRestDoer wraps inner with REST read retry behavior.
func NewRestDoer(inner HTTPDoer, opts ...RestDoerOption) *RestDoer {
	if inner == nil {
		inner = &http.Client{}
	}

	d := &RestDoer{
		inner:              inner,
		headerTimeoutInner: withResponseHeaderTimeout(inner, restPerAttemptTimeout),
		clock:              defaultClock(),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

func (d *RestDoer) Do(req *http.Request) (*http.Response, error) {
	if req == nil {
		return d.inner.Do(req)
	}

	if err := req.Context().Err(); err != nil {
		return nil, err
	}

	if !restEligible(req) {
		return d.inner.Do(req)
	}

	ctx := req.Context()
	original := req.Clone(ctx)

	var lastTransportErr error

	for attempt := 0; attempt < restMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		attemptReq := original.Clone(ctx)
		resp, err := d.doAttempt(ctx, attemptReq)
		if err != nil {
			lastTransportErr = err

			if attempt == restMaxAttempts-1 || ctx.Err() != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				return nil, err
			}

			delay := d.restBackoffDelay(attempt, nil)
			if err := d.clock.sleep(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}

		if !restShouldRetryStatus(resp.StatusCode) || attempt == restMaxAttempts-1 {
			return resp, nil
		}

		discardResponse(resp)

		delay := d.restBackoffDelay(attempt, resp)
		if err := d.clock.sleep(ctx, delay); err != nil {
			return nil, err
		}
	}

	if lastTransportErr != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, lastTransportErr
	}

	return nil, ctx.Err()
}

func (d *RestDoer) doAttempt(ctx context.Context, req *http.Request) (*http.Response, error) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return d.inner.Do(req)
	}

	return d.headerTimeoutInner.Do(req)
}

func restEligible(req *http.Request) bool {
	if req == nil {
		return false
	}

	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return false
	}

	if req.Body != nil && req.Body != http.NoBody {
		return false
	}

	return true
}

func restShouldRetryStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusTooManyRequests:
		return true
	default:
		return false
	}
}

func (d *RestDoer) restBackoffDelay(attempt int, resp *http.Response) time.Duration {
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		if delay, ok := ParseRetryAfter(resp.Header.Get("Retry-After"), d.clock.now(), restMaxRetryAfter); ok {
			return delay
		}
	}

	return fullJitterDelay(attempt, restBaseDelay, restBackoffFactor, restMaxDelay, d.clock.jitter)
}

func discardResponse(resp *http.Response) {
	if resp == nil || resp.Body == nil {
		return
	}

	_, _ = io.CopyN(io.Discard, resp.Body, restDiscardDrainLimit)
	_ = resp.Body.Close()
}

func withResponseHeaderTimeout(inner HTTPDoer, timeout time.Duration) HTTPDoer {
	client, ok := inner.(*http.Client)
	if !ok {
		return inner
	}

	return clientWithResponseHeaderTimeout(client, timeout)
}

func clientWithResponseHeaderTimeout(client *http.Client, timeout time.Duration) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}

	cloned := *client
	transport := cloned.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	t, ok := transport.(*http.Transport)
	if !ok {
		return client
	}

	tc := t.Clone()
	tc.ResponseHeaderTimeout = timeout
	cloned.Transport = tc
	return &cloned
}
