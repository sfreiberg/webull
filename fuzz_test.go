package webull

import (
	"errors"
	"net/http"
	"testing"

	"github.com/sfreiberg/webull/internal/transport"
)

// Error responses are the one payload every request can receive, from either
// of Webull's application shapes, its gateway, or a middlebox that speaks
// neither. Whatever arrives must decode into an *APIError without panicking,
// and the error string must be constructible.
func FuzzDecodeAPIError(f *testing.F) {
	for _, seed := range []string{
		`{"error_code":"OPENAPI_PARAM_ERR","message":"invalid"}`,
		`{"error_msg":"no route"}`,
		`{"timestamp":"2026-09-02","status":404,"error":"Not Found","path":"/openapi/news/summary"}`,
		`<html>gateway timeout</html>`, ``, `null`, `{"message":123}`,
	} {
		f.Add(seed, 500, "req-1", "120")
	}
	f.Fuzz(func(t *testing.T, body string, status int, requestID, retryAfter string) {
		header := http.Header{}
		header.Set("X-Request-Id", requestID)
		header.Set("Retry-After", retryAfter)
		err := decodeAPIError(transport.Response{
			StatusCode: status,
			Header:     header,
			Body:       []byte(body),
		})
		var apiErr *APIError
		if !errors.As(err, &apiErr) || apiErr == nil {
			t.Fatalf("decodeAPIError returned %T", err)
		}
		if apiErr.Error() == "" {
			t.Error("empty error string")
		}
	})
}
