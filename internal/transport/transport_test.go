package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sfreiberg/webull/internal/signing"
)

type stubError struct {
	Status int
	Body   string
}

func (e *stubError) Error() string { return fmt.Sprintf("status %d", e.Status) }

func decodeStub(resp Response) error {
	return &stubError{Status: resp.StatusCode, Body: string(resp.Body)}
}

// newDoer returns a Doer aimed at srv, with sleeping stubbed out so retry
// tests do not spend real time.
func newDoer(t *testing.T, srv *httptest.Server, policy RetryPolicy) *Doer {
	t.Helper()
	return &Doer{
		HTTPClient:  srv.Client(),
		Signer:      signing.New("k", "s"),
		DecodeError: decodeStub,
		Retry:       policy,
		Sleep:       func(context.Context, time.Duration) error { return nil },
	}
}

func hostOf(srv *httptest.Server) string {
	return strings.TrimPrefix(srv.URL, "https://")
}

func TestDoDecodesSuccess(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"ok"}`))
	}))
	defer srv.Close()

	var out struct {
		Name string `json:"name"`
	}
	err := newDoer(t, srv, DefaultRetryPolicy()).Do(context.Background(),
		Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "ok" {
		t.Errorf("out.Name = %q", out.Name)
	}
}

func TestDoAcceptsNilOut(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ignored":true}`))
	}))
	defer srv.Close()

	if err := newDoer(t, srv, DefaultRetryPolicy()).Do(context.Background(),
		Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestDoSignsTheExactBodyItSends(t *testing.T) {
	// The signature covers a digest of the body, so the bytes signed and the
	// bytes transmitted must be identical. Marshalling twice would risk a
	// mismatch.
	var received string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		received = string(b)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	body := map[string]string{"account_id": "A1"}
	err := newDoer(t, srv, DefaultRetryPolicy()).Do(context.Background(),
		Request{Method: "POST", Host: hostOf(srv), Path: "/p", Body: body}, nil)
	if err != nil {
		t.Fatal(err)
	}

	want, _ := json.Marshal(body)
	if received != string(want) {
		t.Errorf("transmitted %q, signed %q", received, want)
	}
}

func TestDoRetriesIdempotentRequests(t *testing.T) {
	var calls int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	err := newDoer(t, srv, DefaultRetryPolicy()).Do(context.Background(),
		Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil)
	if err != nil {
		t.Fatalf("expected success after retries: %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("made %d attempts, want 3", got)
	}
}

// This is the most important test in the package. A retried order placement is
// a duplicated order.
func TestDoNeverRetriesPOST(t *testing.T) {
	var calls int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := newDoer(t, srv, DefaultRetryPolicy()).Do(context.Background(),
		Request{Method: "POST", Host: hostOf(srv), Path: "/orders/place"}, nil)
	if err == nil {
		t.Fatal("expected the error to surface")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("POST was attempted %d times; a replayed order placement can "+
			"duplicate an order and must never be retried", got)
	}
}

func TestDoDoesNotRetryClientErrors(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404, 417} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls int32
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(status)
			}))
			defer srv.Close()

			err := newDoer(t, srv, DefaultRetryPolicy()).Do(context.Background(),
				Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil)
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := atomic.LoadInt32(&calls); got != 1 {
				t.Errorf("retried a %d %d times; replaying cannot change the outcome", status, got)
			}
		})
	}
}

// Retrying a 429 inside a sub-second backoff deepens the throttle rather than
// relieving it, so the SDK surfaces it and lets the caller decide.
func TestDoDoesNotRetryRateLimits(t *testing.T) {
	var calls int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	if err := newDoer(t, srv, DefaultRetryPolicy()).Do(context.Background(),
		Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil); err == nil {
		t.Fatal("expected the rate-limit error to surface")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("429 was retried %d times; that deepens the throttle", got)
	}
}

func TestDoSurfacesErrorAfterExhaustingAttempts(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	err := newDoer(t, srv, RetryPolicy{MaxAttempts: 2}).Do(context.Background(),
		Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil)

	var stub *stubError
	if err == nil {
		t.Fatal("expected an error")
	}
	if !asStub(err, &stub) || stub.Status != http.StatusServiceUnavailable {
		t.Errorf("expected the decoded API error, got %v", err)
	}
}

func asStub(err error, target **stubError) bool {
	return errors.As(err, target)
}

func TestDoPropagatesContextCancellation(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	if err := newDoer(t, srv, RetryPolicy{MaxAttempts: 1}).Do(ctx,
		Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil); err == nil {
		t.Fatal("expected cancellation to surface")
	}
}

func TestDoStopsRetryingWhenContextIsCancelled(t *testing.T) {
	var calls int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	d := newDoer(t, srv, DefaultRetryPolicy())
	d.Sleep = func(context.Context, time.Duration) error {
		cancel()
		return context.Canceled
	}

	if err := d.Do(ctx, Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil); err == nil {
		t.Fatal("expected the cancellation to abort the retry loop")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("kept retrying after cancellation: %d calls", got)
	}
}

func TestDoSendsQueryParameters(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	err := newDoer(t, srv, DefaultRetryPolicy()).Do(context.Background(), Request{
		Method: "GET", Host: hostOf(srv), Path: "/p",
		Query: url.Values{"account_id": {"A1"}, "page_size": {"10"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if gotQuery.Get("account_id") != "A1" || gotQuery.Get("page_size") != "10" {
		t.Errorf("query = %v", gotQuery)
	}
}

func TestDoCapturesRequestID(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req-42")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	var captured string
	d := newDoer(t, srv, RetryPolicy{MaxAttempts: 1})
	d.DecodeError = func(resp Response) error {
		captured = resp.Header.Get("X-Request-Id")
		return &stubError{Status: resp.StatusCode}
	}
	_ = d.Do(context.Background(), Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil)

	if captured != "req-42" {
		t.Errorf("request ID = %q, want req-42", captured)
	}
}

func TestDoRejectsUnmarshalableBody(t *testing.T) {
	d := &Doer{Signer: signing.New("k", "s"), DecodeError: decodeStub}
	err := d.Do(context.Background(), Request{
		Method: "POST", Host: "h", Path: "/p",
		Body: map[string]any{"bad": make(chan int)},
	}, nil)
	if err == nil {
		t.Fatal("expected a marshalling error")
	}
}

func TestIsIdempotent(t *testing.T) {
	for method, want := range map[string]bool{
		"GET": true, "HEAD": true, "OPTIONS": true, "get": true,
		"POST": false, "PUT": false, "PATCH": false, "DELETE": false,
	} {
		if got := isIdempotent(method); got != want {
			t.Errorf("isIdempotent(%q) = %v, want %v", method, got, want)
		}
	}
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	p := RetryPolicy{BaseDelay: 100 * time.Millisecond, MaxDelay: 400 * time.Millisecond}
	for _, tc := range []struct {
		attempt int
		want    time.Duration
	}{
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 400 * time.Millisecond}, // capped
		{99, 400 * time.Millisecond},
	} {
		if got := p.backoff(tc.attempt); got != tc.want {
			t.Errorf("backoff(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}

func TestBackoffZeroValuesUseDefaults(t *testing.T) {
	var p RetryPolicy
	if got := p.backoff(1); got != 200*time.Millisecond {
		t.Errorf("backoff with zero policy = %v", got)
	}
}

func TestDefaultSleepRespectsContext(t *testing.T) {
	d := &Doer{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := d.sleep(ctx, time.Hour); err == nil {
		t.Fatal("expected the cancelled context to abort the sleep")
	}
}

func TestDefaultSleepElapses(t *testing.T) {
	d := &Doer{}
	if err := d.sleep(context.Background(), time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

func TestDoRetriesTransportFailuresForIdempotentOnly(t *testing.T) {
	// A closed server produces a connection error rather than a status.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	client := srv.Client()
	host := hostOf(srv)
	srv.Close()

	var sleeps int32
	newD := func() *Doer {
		return &Doer{
			HTTPClient:  client,
			Signer:      signing.New("k", "s"),
			DecodeError: decodeStub,
			Retry:       RetryPolicy{MaxAttempts: 3},
			Sleep: func(context.Context, time.Duration) error {
				atomic.AddInt32(&sleeps, 1)
				return nil
			},
		}
	}

	if err := newD().Do(context.Background(),
		Request{Method: "GET", Host: host, Path: "/p"}, nil); err == nil {
		t.Fatal("expected a transport error")
	}
	if got := atomic.LoadInt32(&sleeps); got != 2 {
		t.Errorf("GET backed off %d times, want 2 retries", got)
	}

	atomic.StoreInt32(&sleeps, 0)
	if err := newD().Do(context.Background(),
		Request{Method: "POST", Host: host, Path: "/orders/place"}, nil); err == nil {
		t.Fatal("expected a transport error")
	}
	if got := atomic.LoadInt32(&sleeps); got != 0 {
		t.Errorf("POST retried %d times after a transport failure; the server may "+
			"have processed the request, so it must not be replayed", got)
	}
}

func TestDoTreatsZeroMaxAttemptsAsOne(t *testing.T) {
	var calls int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := newDoer(t, srv, RetryPolicy{})
	if err := d.Do(context.Background(),
		Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil); err == nil {
		t.Fatal("expected an error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("made %d attempts, want 1", got)
	}
}

func TestDoPreservesAPIErrorWhenContextExpiresDuringBackoff(t *testing.T) {
	// A caller whose deadline expires during the retry sleep must still be
	// able to see why the SDK was retrying: the *APIError from the previous
	// attempt carries the status, code and request ID.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	d := newDoer(t, srv, DefaultRetryPolicy())
	d.Sleep = func(context.Context, time.Duration) error { return context.DeadlineExceeded }

	err := d.Do(context.Background(), Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("context error was lost: %v", err)
	}
	var stub *stubError
	if !errors.As(err, &stub) || stub.Status != http.StatusServiceUnavailable {
		t.Errorf("the API error from the failed attempt was discarded: %v", err)
	}
}

func TestDoReportsOversizedSuccessResponse(t *testing.T) {
	// Silent truncation would surface as a baffling JSON parse error pointing
	// the user at a decoding bug rather than a size limit.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":"`))
		filler := bytes.Repeat([]byte("x"), 64<<10)
		for written := 0; written <= maxResponseBody; written += len(filler) {
			_, _ = w.Write(filler)
		}
		_, _ = w.Write([]byte(`"}`))
	}))
	defer srv.Close()

	var out map[string]string
	err := newDoer(t, srv, RetryPolicy{MaxAttempts: 1}).Do(context.Background(),
		Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, &out)
	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("got %v, want ErrResponseTooLarge", err)
	}
}

func TestDoMarksTruncatedErrorBodies(t *testing.T) {
	big := append([]byte(`{"message":"`), bytes.Repeat([]byte("e"), maxErrorBody*2)...)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(append(big, []byte(`"}`)...))
	}))
	defer srv.Close()

	var got Response
	d := newDoer(t, srv, RetryPolicy{MaxAttempts: 1})
	d.DecodeError = func(resp Response) error {
		got = resp
		return &stubError{Status: resp.StatusCode}
	}
	_ = d.Do(context.Background(), Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil)

	if !got.Truncated {
		t.Error("an oversized error body must be flagged as truncated")
	}
	if len(got.Body) != maxErrorBody {
		t.Errorf("body length = %d, want exactly %d", len(got.Body), maxErrorBody)
	}
}

func TestDoOmitsContentTypeOnBodylessRequests(t *testing.T) {
	var contentType string
	var present bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType, present = r.Header.Get("Content-Type"), r.Header.Values("Content-Type") != nil
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	d := newDoer(t, srv, RetryPolicy{MaxAttempts: 1})
	if err := d.Do(context.Background(), Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil); err != nil {
		t.Fatal(err)
	}
	if present {
		t.Errorf("bodyless GET carried Content-Type %q; some gateways reject that", contentType)
	}

	if err := d.Do(context.Background(), Request{Method: "POST", Host: hostOf(srv), Path: "/p",
		Body: map[string]int{"a": 1}}, nil); err != nil {
		t.Fatal(err)
	}
	if contentType != "application/json" {
		t.Errorf("POST with a body should carry application/json, got %q", contentType)
	}
}

func TestGetAndPostHelpers(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if r.URL.Query().Get("k") != "v" {
				t.Errorf("query = %v", r.URL.Query())
			}
			_, _ = w.Write([]byte(`{"ok":"get"}`))
		case http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			if string(body) != `{"in":"post"}` {
				t.Errorf("body = %s", body)
			}
			_, _ = w.Write([]byte(`{"ok":"post"}`))
		}
	}))
	t.Cleanup(srv.Close)
	d := newDoer(t, srv, DefaultRetryPolicy())

	var out struct {
		OK string `json:"ok"`
	}
	if err := d.Get(context.Background(), hostOf(srv), "/x", url.Values{"k": {"v"}}, &out); err != nil || out.OK != "get" {
		t.Errorf("Get: %v %+v", err, out)
	}
	if err := d.Post(context.Background(), hostOf(srv), "/x", map[string]string{"in": "post"}, &out); err != nil || out.OK != "post" {
		t.Errorf("Post: %v %+v", err, out)
	}
}
