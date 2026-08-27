package webull

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sfreiberg/webull/internal/transport"
)

// headerWithRequestID builds a header carrying a request id, or an empty one.
func headerWithRequestID(id string) http.Header {
	h := http.Header{}
	if id != "" {
		h.Set("X-Request-Id", id)
	}
	return h
}

func TestAPIErrorMatchesSentinels(t *testing.T) {
	for _, tc := range []struct {
		status   int
		sentinel error
	}{
		{http.StatusUnauthorized, ErrAuthentication},
		{http.StatusForbidden, ErrPermission},
		{http.StatusBadRequest, ErrInvalidRequest},
		{http.StatusExpectationFailed, ErrInvalidRequest}, // Webull uses 417 for parameter errors
		{http.StatusNotFound, ErrNotFound},
		{http.StatusTooManyRequests, ErrRateLimited},
		{http.StatusInternalServerError, ErrServer},
		{http.StatusBadGateway, ErrServer},
	} {
		err := error(&APIError{StatusCode: tc.status})
		if !errors.Is(err, tc.sentinel) {
			t.Errorf("HTTP %d: errors.Is(err, %v) = false, want true", tc.status, tc.sentinel)
		}
	}
}

func TestAPIErrorDoesNotMatchUnrelatedSentinels(t *testing.T) {
	err := error(&APIError{StatusCode: http.StatusForbidden})

	if errors.Is(err, ErrAuthentication) {
		t.Error("403 must not match ErrAuthentication; callers branch on the difference")
	}
	if errors.Is(err, ErrRateLimited) {
		t.Error("403 must not match ErrRateLimited")
	}
}

func TestAPIErrorAsExposesDetail(t *testing.T) {
	err := error(&APIError{
		StatusCode: http.StatusExpectationFailed,
		Code:       "OPENAPI_PARAM_ERR",
		Message:    "invalid page_size",
		RequestID:  "req-1",
	})

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("errors.As did not unwrap to *APIError")
	}
	if apiErr.Code != "OPENAPI_PARAM_ERR" || apiErr.RequestID != "req-1" {
		t.Errorf("unexpected detail: %+v", apiErr)
	}
}

func TestAPIErrorMessages(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  *APIError
		want []string
	}{
		{"code and message", &APIError{StatusCode: 417, Code: "E", Message: "bad"}, []string{"bad", "417", "E"}},
		{"message only", &APIError{StatusCode: 404, Message: "404 Route Not Found"}, []string{"404 Route Not Found"}},
		{"neither", &APIError{StatusCode: 500}, []string{"unexpected response", "500"}},
	} {
		got := tc.err.Error()
		for _, want := range tc.want {
			if !strings.Contains(got, want) {
				t.Errorf("%s: Error() = %q, missing %q", tc.name, got, want)
			}
		}
	}
}

func TestAPIErrorTemporary(t *testing.T) {
	for status, want := range map[int]bool{
		http.StatusTooManyRequests:     true,
		http.StatusInternalServerError: true,
		http.StatusServiceUnavailable:  true,
		http.StatusBadRequest:          false,
		http.StatusUnauthorized:        false,
		http.StatusForbidden:           false,
		http.StatusNotFound:            false,
	} {
		if got := (&APIError{StatusCode: status}).Temporary(); got != want {
			t.Errorf("HTTP %d: Temporary() = %v, want %v", status, got, want)
		}
	}
}

func TestDecodeAPIErrorHandlesBothShapes(t *testing.T) {
	t.Run("application error", func(t *testing.T) {
		err := decodeAPIError(transport.Response{StatusCode: 417, Header: headerWithRequestID("rid"), Body: []byte(`{"message":"invalid page_size","error_code":"OPENAPI_PARAM_ERR"}`)})
		var apiErr *APIError
		errors.As(err, &apiErr)
		if apiErr.Code != "OPENAPI_PARAM_ERR" || apiErr.Message != "invalid page_size" {
			t.Errorf("got %+v", apiErr)
		}
	})

	t.Run("gateway error uses a different key", func(t *testing.T) {
		err := decodeAPIError(transport.Response{StatusCode: 404, Header: headerWithRequestID(""), Body: []byte(`{"error_msg":"404 Route Not Found"}`)})
		var apiErr *APIError
		errors.As(err, &apiErr)
		if apiErr.Message != "404 Route Not Found" {
			t.Errorf("error_msg was not read: %+v", apiErr)
		}
		if apiErr.Code != "" {
			t.Errorf("gateway errors carry no code, got %q", apiErr.Code)
		}
	})

	t.Run("unparseable body is preserved", func(t *testing.T) {
		err := decodeAPIError(transport.Response{StatusCode: 502, Header: headerWithRequestID(""), Body: []byte("<html>gateway</html>")})
		var apiErr *APIError
		errors.As(err, &apiErr)
		if !strings.Contains(string(apiErr.Body), "gateway") {
			t.Error("raw body should be retained when it cannot be parsed")
		}
		if !errors.Is(err, ErrServer) {
			t.Error("status classification should not depend on parsing the body")
		}
	})
}

func TestAPIErrorIsRejectsUnknownTarget(t *testing.T) {
	err := &APIError{StatusCode: http.StatusForbidden}
	if err.Is(errors.New("some other error")) {
		t.Error("Is must return false for unrelated targets")
	}
}

func TestDecodeAPIErrorKeepsMessageWhenAnotherFieldMismatches(t *testing.T) {
	// encoding/json populates what it understood before returning a type
	// error. A mismatch on error_code must not discard a message that decoded
	// perfectly well.
	err := decodeAPIError(transport.Response{
		StatusCode: 400,
		Header:     http.Header{},
		Body:       []byte(`{"error_code":123,"message":"a real message"}`),
	})

	var apiErr *APIError
	errors.As(err, &apiErr)
	if apiErr.Message != "a real message" {
		t.Errorf("message was discarded: %+v", apiErr)
	}
}

func TestDecodeAPIErrorReadsRetryAfterSeconds(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "30")

	err := decodeAPIError(transport.Response{StatusCode: 429, Header: h, Body: []byte(`{}`)})

	var apiErr *APIError
	errors.As(err, &apiErr)
	if apiErr.RetryAfter != 30*time.Second {
		t.Errorf("RetryAfter = %v, want 30s", apiErr.RetryAfter)
	}
}

func TestDecodeAPIErrorReadsRetryAfterHTTPDate(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", time.Now().Add(90*time.Second).UTC().Format(http.TimeFormat))

	err := decodeAPIError(transport.Response{StatusCode: 429, Header: h, Body: []byte(`{}`)})

	var apiErr *APIError
	errors.As(err, &apiErr)
	if apiErr.RetryAfter <= 0 || apiErr.RetryAfter > 91*time.Second {
		t.Errorf("RetryAfter = %v, want roughly 90s", apiErr.RetryAfter)
	}
}

func TestDecodeAPIErrorIgnoresUnusableRetryAfter(t *testing.T) {
	for _, v := range []string{"", "not-a-number", "-5", "Mon, 02 Jan 2006 15:04:05 GMT"} {
		h := http.Header{}
		if v != "" {
			h.Set("Retry-After", v)
		}
		err := decodeAPIError(transport.Response{StatusCode: 429, Header: h, Body: []byte(`{}`)})

		var apiErr *APIError
		errors.As(err, &apiErr)
		if apiErr.RetryAfter != 0 {
			t.Errorf("Retry-After %q gave %v, want 0", v, apiErr.RetryAfter)
		}
	}
}

func TestDecodeAPIErrorRecordsTruncation(t *testing.T) {
	err := decodeAPIError(transport.Response{
		StatusCode: 500,
		Header:     http.Header{},
		Body:       []byte(`{"message":"cut off`),
		Truncated:  true,
	})

	var apiErr *APIError
	errors.As(err, &apiErr)
	if !apiErr.Truncated {
		t.Error("truncation should be visible to the caller, so an empty Code " +
			"is not mistaken for an unrecognised error shape")
	}
}

func TestAPIErrorCodeAccessor(t *testing.T) {
	err := &APIError{Code: "OPENAPI_PARAM_ERR"}
	if err.ErrorCode() != "OPENAPI_PARAM_ERR" {
		t.Errorf("ErrorCode() = %q", err.ErrorCode())
	}
	// The accessor is what lets other packages classify by code without
	// importing this one.
	var coded interface{ ErrorCode() string }
	if !errors.As(error(err), &coded) {
		t.Error("APIError should satisfy the ErrorCode interface through errors.As")
	}
}
