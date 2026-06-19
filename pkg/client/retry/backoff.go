package retry

import (
	"context"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

const (
	restMaxAttempts       = 5
	restBaseDelay         = 250 * time.Millisecond
	restBackoffFactor     = 2.0
	restMaxDelay          = 5 * time.Second
	restMaxRetryAfter     = 5 * time.Second
	restPerAttemptTimeout = 30 * time.Second
	restDiscardDrainLimit = 64 * 1024
)

type clock struct {
	now    func() time.Time
	sleep  func(context.Context, time.Duration) error
	jitter func(int64) int64
}

func defaultClock() clock {
	return clock{
		now:    time.Now,
		sleep:  sleepContext,
		jitter: rand.Int64N,
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func fullJitterDelay(attempt int, base time.Duration, factor float64, maxDelay time.Duration, jitter func(int64) int64) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	delayLimit := float64(base) * math.Pow(factor, float64(attempt))
	if delayLimit > float64(maxDelay) {
		delayLimit = float64(maxDelay)
	}
	if delayLimit <= 0 {
		return 0
	}

	return time.Duration(jitter(int64(delayLimit) + 1))
}

// ParseRetryAfter parses an HTTP Retry-After header value.
// It supports delta-seconds and HTTP-date forms. Past HTTP-date values yield zero delay.
// Returns ok=false when the header is missing, invalid, negative, or exceeds max.
func ParseRetryAfter(header string, now time.Time, maxDelay time.Duration) (time.Duration, bool) {
	if header == "" {
		return 0, false
	}

	if seconds, err := strconv.Atoi(header); err == nil {
		if seconds < 0 {
			return 0, false
		}

		delay := time.Duration(seconds) * time.Second
		if delay > maxDelay {
			return 0, false
		}

		return delay, true
	}

	retryAt, err := http.ParseTime(header)
	if err != nil {
		return 0, false
	}

	delay := retryAt.Sub(now)
	if delay < 0 {
		delay = 0
	}
	if delay > maxDelay {
		return 0, false
	}

	return delay, true
}
