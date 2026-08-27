package webull

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for conditions callers commonly branch on. Match them with
// errors.Is; the underlying *APIError remains reachable with errors.As.
var (
	// ErrAuthentication indicates the request was not authenticated: a missing,
	// malformed or rejected signature, or absent credentials.
	ErrAuthentication = errors.New("webull: authentication failed")

	// ErrPermission indicates valid credentials that lack access to the
	// requested resource, such as an account the key cannot see or a market
	// data tier the key is not entitled to.
	ErrPermission = errors.New("webull: permission denied")

	// ErrInvalidRequest indicates the request was rejected as malformed or
	// carrying invalid parameters.
	ErrInvalidRequest = errors.New("webull: invalid request")

	// ErrNotFound indicates the endpoint or resource does not exist.
	ErrNotFound = errors.New("webull: not found")

	// ErrRateLimited indicates the request exceeded a rate limit.
	ErrRateLimited = errors.New("webull: rate limited")

	// ErrServer indicates Webull reported a server-side failure.
	ErrServer = errors.New("webull: server error")
)

// statusExpectationFailed is HTTP 417, which Webull returns for parameter
// validation failures rather than the more conventional 400.
const statusExpectationFailed = http.StatusExpectationFailed

// APIError is a structured error response from Webull.
//
// Webull returns two different error shapes. Application errors carry a
// machine-readable code and a message; errors produced by the API gateway,
// such as an unrouted path, carry only a message under a different key. Both
// decode into this type, and Code is empty for the latter.
type APIError struct {
	// StatusCode is the HTTP status.
	StatusCode int
	// Code is Webull's machine-readable error code, such as
	// "OPENAPI_PARAM_ERR". Empty for gateway-level errors.
	Code string
	// Message is Webull's human-readable description.
	Message string
	// RequestID correlates the request with Webull support, when the response
	// carries one.
	RequestID string
	// Body is the raw response body, bounded in size. It is retained because
	// Webull's error catalogue is not exhaustively documented, and the raw
	// payload is often the only way to diagnose an unfamiliar failure.
	Body []byte
}

// Error implements error.
func (e *APIError) Error() string {
	switch {
	case e.Code != "" && e.Message != "":
		return fmt.Sprintf("webull: %s (HTTP %d, code %s)", e.Message, e.StatusCode, e.Code)
	case e.Message != "":
		return fmt.Sprintf("webull: %s (HTTP %d)", e.Message, e.StatusCode)
	default:
		return fmt.Sprintf("webull: unexpected response (HTTP %d)", e.StatusCode)
	}
}

// Is reports whether this error matches one of the package sentinels, so that
// errors.Is(err, ErrPermission) works without callers inspecting status codes.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrAuthentication:
		return e.StatusCode == http.StatusUnauthorized
	case ErrPermission:
		return e.StatusCode == http.StatusForbidden
	case ErrInvalidRequest:
		return e.StatusCode == http.StatusBadRequest || e.StatusCode == statusExpectationFailed
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrRateLimited:
		return e.StatusCode == http.StatusTooManyRequests
	case ErrServer:
		return e.StatusCode >= 500
	}
	return false
}

// Temporary reports whether retrying the same request might succeed.
//
// It is deliberately conservative: it never reports true for a client error,
// because replaying a rejected request produces the same rejection. Callers
// must not treat a true result as licence to retry a non-idempotent request
// such as order placement without first reconciling.
func (e *APIError) Temporary() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
}
