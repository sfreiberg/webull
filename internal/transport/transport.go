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
	"errors"
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

// maxResponseBody bounds a successful response. Exceeding it is reported as an
// error rather than silently truncated, because a truncated body would surface
// as a confusing JSON parse failure.
const maxResponseBody = 32 << 20

// ErrResponseTooLarge is returned when a response exceeds maxResponseBody.
var ErrResponseTooLarge = errors.New("webull: response body exceeds the maximum size")

// Response is a failed HTTP response handed to an ErrorDecoder.
type Response struct {
	StatusCode int
	Header     http.Header
	// Body is the response body, truncated to maxErrorBody.
	Body []byte
	// Truncated reports whether Body was cut short, so a decoder does not
	// mistake unparseable truncated JSON for an unrecognised error shape.
	Truncated bool
}

// ErrorDecoder turns a failed HTTP response into an error. The SDK supplies
// this so that transport need not import the public error types, which would
// create an import cycle.
type ErrorDecoder func(Response) error

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
	// Authorizer, if set, returns headers to add to each request after
	// signing — an OAuth bearer token for the Connect API, which is both
	// signed and bearer-authenticated. It is called per request so a
	// rotating token is always current, and its error fails the request
	// before anything is sent.
	Authorizer func(context.Context) (map[string]string, error)
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
	// Form, when non-nil, is sent as an application/x-www-form-urlencoded
	// body instead of JSON — the Connect API's token endpoint requires it.
	// Body and Form are mutually exclusive.
	Form url.Values
}

// Do executes req and decodes a successful response into out, which may be nil
// if the caller does not need the body.
func (d *Doer) Do(ctx context.Context, req Request, out any) error {
	var body []byte
	switch {
	case req.Form != nil:
		body = []byte(req.Form.Encode())
	case req.Body != nil:
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
				// Preserve why we were retrying. Returning only the context
				// error would discard the *APIError from the previous attempt,
				// leaving the caller unable to tell a timeout during backoff
				// from a timeout with no server response at all.
				if lastErr != nil {
					return errors.Join(err, lastErr)
				}
				return err
			}
		}

		resp, err := d.attempt(ctx, req, body)
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

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out == nil {
				return nil
			}
			if len(resp.Body) == 0 {
				// A caller that asked for a decoded result must not silently
				// receive a zero value. Reading a flag such as
				// token_check_enabled from an empty body would report false,
				// which is indistinguishable from the server saying false.
				return fmt.Errorf("webull: expected a response body, got none (HTTP %d)", resp.StatusCode)
			}
			if err := json.Unmarshal(resp.Body, out); err != nil {
				return fmt.Errorf("webull: decoding response: %w", err)
			}
			return nil
		}

		apiErr := d.DecodeError(resp)
		lastErr = apiErr
		if d.Retry.retryStatus(resp.StatusCode, req.Method) && attempt < attempts-1 {
			continue
		}
		return apiErr
	}
	return lastErr
}

// attempt performs a single signed request.
func (d *Doer) attempt(ctx context.Context, req Request, body []byte) (Response, error) {
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
		return Response{}, fmt.Errorf("webull: building request: %w", err)
	}

	for k, v := range d.Signer.Sign(signing.Request{
		Host:  req.Host,
		Path:  req.Path,
		Query: req.Query,
		Body:  body,
	}) {
		httpReq.Header.Set(k, v)
	}
	if d.Authorizer != nil {
		headers, err := d.Authorizer(ctx)
		if err != nil {
			return Response{}, fmt.Errorf("webull: authorizing request: %w", err)
		}
		for k, v := range headers {
			httpReq.Header.Set(k, v)
		}
	}
	// Only set on requests that actually carry a body: a Content-Type on a
	// bodyless GET is meaningless and some gateways reject it.
	if len(body) > 0 {
		if req.Form != nil {
			httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			httpReq.Header.Set("Content-Type", "application/json")
		}
	}
	httpReq.Header.Set("Accept", "application/json")
	if d.UserAgent != "" {
		httpReq.Header.Set("User-Agent", d.UserAgent)
	}

	resp, err := d.HTTPClient.Do(httpReq)
	if err != nil {
		// The URL is included by net/http in this error but the headers, which
		// hold the signature, are not.
		return Response{}, fmt.Errorf("webull: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	limit := int64(maxErrorBody)
	if success {
		limit = maxResponseBody
	}

	// Read one byte past the limit so that hitting it is detectable rather
	// than silently truncating.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return Response{}, fmt.Errorf("webull: reading response: %w", err)
	}

	truncated := int64(len(respBody)) > limit
	if truncated {
		respBody = respBody[:limit]
		// Drain the remainder so the connection can be reused rather than
		// being closed and re-established.
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBody))
	}

	if success && truncated {
		return Response{}, fmt.Errorf("%w (limit %d bytes)", ErrResponseTooLarge, maxResponseBody)
	}

	return Response{
		StatusCode: resp.StatusCode,
		Header:     resp.Header,
		Body:       respBody,
		Truncated:  truncated,
	}, nil
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
