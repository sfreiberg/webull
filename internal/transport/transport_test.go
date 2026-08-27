package transport

import (
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

func decodeStub(status int, _ string, body []byte) error {
	return &stubError{Status: status, Body: string(body)}
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
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestDoRetriesRateLimitForIdempotent(t *testing.T) {
	var calls int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	if err := newDoer(t, srv, DefaultRetryPolicy()).Do(context.Background(),
		Request{Method: "GET", Host: hostOf(srv), Path: "/p"}, nil); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Error("expected one retry after 429")
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
	d.DecodeError = func(status int, requestID string, body []byte) error {
		captured = requestID
		return &stubError{Status: status}
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

func TestTrimBodyBoundsSize(t *testing.T) {
	if got := TrimBody([]byte("short")); string(got) != "short" {
		t.Errorf("short bodies should pass through, got %q", got)
	}
	big := make([]byte, maxErrorBody*2)
	if got := TrimBody(big); len(got) <= maxErrorBody || len(got) > maxErrorBody+4 {
		t.Errorf("trimmed length = %d, want just over %d", len(got), maxErrorBody)
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
