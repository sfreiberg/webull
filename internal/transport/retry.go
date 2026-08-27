package transport

import (
	"math"
	"net/http"
	"time"
)

// RetryPolicy controls whether and how a failed request is retried.
//
// The default is deliberately conservative. A trading API is the wrong place
// for optimistic retries: replaying a request whose outcome is unknown can
// duplicate an order. Only requests that are safe to replay are retried, and
// only for failures that plausibly resolve on their own.
type RetryPolicy struct {
	// MaxAttempts is the total number of attempts, not the number of retries.
	// Values below 1 are treated as 1.
	MaxAttempts int
	// BaseDelay is the first backoff interval; each subsequent attempt doubles
	// it up to MaxDelay.
	BaseDelay time.Duration
	// MaxDelay caps the backoff.
	MaxDelay time.Duration
}

// DefaultRetryPolicy retries twice more after an initial failure, for
// idempotent requests only.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: 3,
		BaseDelay:   200 * time.Millisecond,
		MaxDelay:    2 * time.Second,
	}
}

// retryStatus reports whether a response status warrants another attempt.
func (p RetryPolicy) retryStatus(status int, method string) bool {
	if !isIdempotent(method) {
		return false
	}
	return status == http.StatusTooManyRequests || status >= 500
}

// retryTransport reports whether a transport failure warrants another attempt.
//
// A transport failure is more dangerous than an error status, because the
// request may have been received and processed even though no response
// arrived. Only idempotent requests are replayed.
func (p RetryPolicy) retryTransport(method string) bool {
	return isIdempotent(method)
}

// backoff returns the delay before the given attempt, counting from 1.
func (p RetryPolicy) backoff(attempt int) time.Duration {
	base := p.BaseDelay
	if base <= 0 {
		base = 200 * time.Millisecond
	}
	maxDelay := p.MaxDelay
	if maxDelay <= 0 {
		maxDelay = 2 * time.Second
	}

	d := time.Duration(float64(base) * math.Pow(2, float64(attempt-1)))
	if d > maxDelay || d <= 0 {
		return maxDelay
	}
	return d
}
