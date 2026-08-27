// Package transport carries out signed HTTP requests against Webull and turns
// responses into typed results or errors.
//
// It is internal: the retry policy, error decoding and signing hand-off are
// implementation details that should be free to change without breaking the
// SDK's public API.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sfreiberg/webull/internal/signing"
)

// maxErrorBody bounds how much of an error response is retained. Enough to
// diagnose an unfamiliar failure, not enough to fill logs with a stray HTML
// error page.
const maxErrorBody = 8 << 10

// ErrorDecoder turns a failed HTTP response into an error. The SDK supplies
// this so that transport need not import the public error types, which would
// create an import cycle.
type ErrorDecoder func(status int, requestID string, body []byte) error

// Doer performs signed requests.
type Doer struct {
	HTTPClient  *http.Client
	Signer      *signing.Signer
	UserAgent   string
	DecodeError ErrorDecoder
	Retry       RetryPolicy
	// Sleep pauses between retry attempts. Defaults to time.Sleep; tests
	// replace it to avoid real delays.
	Sleep func(context.Context, time.Duration) error
}

// Request describes one call.
type Request struct {
	Method string
	Host   string
	Path   string
	Query  url.Values
	// Body is marshalled to JSON if non-nil. The marshalled bytes are both
	// signed and transmitted, so the signature always covers exactly what is
	// sent.
	Body any
}

// Do executes req and decodes a successful response into out, which may be nil
// if the caller does not need the body.
func (d *Doer) Do(ctx context.Context, req Request, out any) error {
	var body []byte
	if req.Body != nil {
		var err error
		body, err = json.Marshal(req.Body)
		if err != nil {
			return fmt.Errorf("webull: encoding request body: %w", err)
		}
	}

	attempts := d.Retry.MaxAttempts
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if attempt > 0 {
			if err := d.sleep(ctx, d.Retry.backoff(attempt)); err != nil {
				return err
			}
		}

		status, respBody, requestID, err := d.attempt(ctx, req, body)
		if err != nil {
			// A transport-level failure. Retrying is only safe when the
			// request is idempotent, because the server may have processed a
			// request whose response we never saw.
			lastErr = err
			if d.Retry.retryTransport(req.Method) && attempt < attempts-1 {
				continue
			}
			return err
		}

		if status >= 200 && status < 300 {
			if out == nil || len(respBody) == 0 {
				return nil
			}
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("webull: decoding response: %w", err)
			}
			return nil
		}

		apiErr := d.DecodeError(status, requestID, respBody)
		lastErr = apiErr
		if d.Retry.retryStatus(status, req.Method) && attempt < attempts-1 {
			continue
		}
		return apiErr
	}
	return lastErr
}

// attempt performs a single signed request.
func (d *Doer) attempt(ctx context.Context, req Request, body []byte) (int, []byte, string, error) {
	u := url.URL{Scheme: "https", Host: req.Host, Path: req.Path}
	if len(req.Query) > 0 {
		u.RawQuery = req.Query.Encode()
	}

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, u.String(), reader)
	if err != nil {
		return 0, nil, "", fmt.Errorf("webull: building request: %w", err)
	}

	for k, v := range d.Signer.Sign(signing.Request{
		Host:  req.Host,
		Path:  req.Path,
		Query: req.Query,
		Body:  body,
	}) {
		httpReq.Header.Set(k, v)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")
	if d.UserAgent != "" {
		httpReq.Header.Set("User-Agent", d.UserAgent)
	}

	resp, err := d.HTTPClient.Do(httpReq)
	if err != nil {
		// The URL is included by net/http in this error but the headers, which
		// hold the signature, are not.
		return 0, nil, "", fmt.Errorf("webull: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	limit := int64(maxErrorBody)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		limit = 1 << 24 // 16 MiB, ample for any documented response
	}
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return 0, nil, "", fmt.Errorf("webull: reading response: %w", err)
	}

	return resp.StatusCode, respBody, requestIDOf(resp), nil
}

// requestIDOf extracts a correlation identifier if the response carries one.
func requestIDOf(resp *http.Response) string {
	for _, h := range []string{"X-Request-Id", "X-Request-ID", "Request-Id"} {
		if v := resp.Header.Get(h); v != "" {
			return v
		}
	}
	return ""
}

func (d *Doer) sleep(ctx context.Context, dur time.Duration) error {
	if d.Sleep != nil {
		return d.Sleep(ctx, dur)
	}
	t := time.NewTimer(dur)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// TrimBody bounds a body for inclusion in an error.
func TrimBody(b []byte) []byte {
	if len(b) <= maxErrorBody {
		return b
	}
	return append(bytes.Clone(b[:maxErrorBody]), []byte("…")...)
}

// isIdempotent reports whether replaying the method is safe. POST is excluded:
// in this API it places, replaces and cancels orders.
func isIdempotent(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}
