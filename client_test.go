package webull

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient starts a TLS test server and returns a Client pointed at it.
// The transport always speaks https, so a plain httptest server would not do.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	c, err := NewClient(Config{
		AppKey:      "test-key",
		AppSecret:   "test-secret",
		Environment: Sandbox,
		HTTPClient:  srv.Client(),
		EndpointOverrides: map[string]string{
			"trading": strings.TrimPrefix(srv.URL, "https://"),
		},
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c, srv
}

func TestNewClientValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want error
	}{
		{"missing key", Config{AppSecret: "s", Environment: Sandbox}, ErrMissingCredentials},
		{"missing secret", Config{AppKey: "k", Environment: Sandbox}, ErrMissingCredentials},
		{"missing environment", Config{AppKey: "k", AppSecret: "s"}, nil},
		{"unknown environment", Config{AppKey: "k", AppSecret: "s", Environment: "staging"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewClient(tc.cfg)
			if err == nil {
				t.Fatal("expected an error")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Errorf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestNewClientErrorsNeverContainCredentials(t *testing.T) {
	const secret = "super-secret"
	_, err := NewClient(Config{AppKey: "k", AppSecret: secret, Environment: "nope"})
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatal("error message leaked the app secret")
	}
}

func TestNewClientRequiresExplicitEnvironment(t *testing.T) {
	// There is deliberately no default. Defaulting to sandbox would silently
	// send real orders nowhere; defaulting to production would silently send
	// them somewhere very real.
	if _, err := NewClient(Config{AppKey: "k", AppSecret: "s"}); err == nil {
		t.Fatal("environment must be required")
	}
}

func TestClientSignsEveryRequest(t *testing.T) {
	var got http.Header
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte(`{"token_check_enabled":false}`))
	})

	if _, err := c.TokenCheckEnabled(context.Background()); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	for _, h := range []string{
		"x-app-key", "x-timestamp", "x-signature-version",
		"x-signature-algorithm", "x-signature-nonce", "x-signature",
	} {
		if got.Get(h) == "" {
			t.Errorf("missing signature header %s", h)
		}
	}
	if got.Get("x-signature-algorithm") != "HMAC-SHA256" {
		t.Errorf("algorithm = %q, want HMAC-SHA256", got.Get("x-signature-algorithm"))
	}
	if !strings.Contains(got.Get("User-Agent"), "webull-go/") {
		t.Errorf("User-Agent = %q, want it to identify the SDK", got.Get("User-Agent"))
	}
}

func TestClientNeverSendsTheSecret(t *testing.T) {
	const secret = "do-not-transmit"
	var seen http.Header
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		AppKey: "k", AppSecret: secret, Environment: Sandbox,
		HTTPClient:        srv.Client(),
		EndpointOverrides: map[string]string{"trading": strings.TrimPrefix(srv.URL, "https://")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.TokenCheckEnabled(context.Background()); err != nil {
		t.Fatal(err)
	}

	for k, vs := range seen {
		for _, v := range vs {
			if strings.Contains(v, secret) {
				t.Fatalf("header %s transmitted the app secret", k)
			}
		}
	}
}

func TestClientDecodesErrors(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusExpectationFailed)
		_, _ = w.Write([]byte(`{"message":"invalid page_size","error_code":"OPENAPI_PARAM_ERR"}`))
	})

	_, err := c.TokenCheckEnabled(context.Background())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, ErrInvalidRequest) {
		t.Errorf("417 should classify as ErrInvalidRequest, got %v", err)
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "OPENAPI_PARAM_ERR" {
		t.Errorf("expected the error code to survive: %v", err)
	}
}

func TestClientHonoursContextCancellation(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := c.TokenCheckEnabled(ctx); err == nil {
		t.Fatal("expected cancellation to surface as an error")
	}
}

func TestClientRejectsMalformedJSON(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{not json`))
	})

	if _, err := c.TokenCheckEnabled(context.Background()); err == nil {
		t.Fatal("expected a decode error")
	}
}

func TestClientEnvironmentAccessor(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{}`)) })
	if c.Environment() != Sandbox {
		t.Errorf("Environment() = %v, want sandbox", c.Environment())
	}
}

func TestTokenCheckEnabledReadsTheFlag(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/openapi/config" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"token_check_enabled":true}`))
	})

	enabled, err := c.TokenCheckEnabled(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Error("expected the flag to be reported as true")
	}
}

func TestClientAppendsCallerUserAgent(t *testing.T) {
	var got string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		AppKey: "k", AppSecret: "s", Environment: Sandbox,
		UserAgent:         "my-app/2.1",
		HTTPClient:        srv.Client(),
		EndpointOverrides: map[string]string{"trading": strings.TrimPrefix(srv.URL, "https://")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.TokenCheckEnabled(context.Background()); err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(got, "my-app/2.1 ") {
		t.Errorf("User-Agent = %q, want the caller's token first", got)
	}
	if !strings.Contains(got, "webull-go/") {
		t.Errorf("User-Agent = %q, want the SDK still identified", got)
	}
}

func TestClientSurfacesHostResolutionErrors(t *testing.T) {
	// A client whose environment is corrupted after construction must fail the
	// request rather than send it somewhere unintended.
	c := &Client{cfg: Config{Environment: "staging"}}
	if _, err := c.TokenCheckEnabled(context.Background()); err == nil {
		t.Fatal("expected host resolution to fail")
	}
}

func TestClientRefusesRedirects(t *testing.T) {
	// Go strips only Authorization and Cookie across hosts, so following a
	// redirect would forward the app key and signature to the target.
	var hits int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		if r.Header.Get("x-app-key") != "" {
			t.Error("signature headers were forwarded to the redirect target")
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer target.Close()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, target.URL, http.StatusFound)
	}))
	defer srv.Close()

	c, err := NewClient(Config{
		AppKey: "k", AppSecret: "s", Environment: Sandbox,
		HTTPClient:        srv.Client(),
		EndpointOverrides: map[string]string{"trading": strings.TrimPrefix(srv.URL, "https://")},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.TokenCheckEnabled(context.Background()); err == nil {
		t.Fatal("expected the redirect to be refused")
	} else if !errors.Is(err, ErrRedirectNotAllowed) {
		t.Errorf("got %v, want ErrRedirectNotAllowed", err)
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("the redirect target was contacted %d times", got)
	}
}

func TestClientDoesNotMutateCallerHTTPClient(t *testing.T) {
	caller := &http.Client{Timeout: time.Minute}
	_, err := NewClient(Config{
		AppKey: "k", AppSecret: "s", Environment: Sandbox, HTTPClient: caller,
	})
	if err != nil {
		t.Fatal(err)
	}
	if caller.CheckRedirect != nil {
		t.Error("NewClient mutated the caller's http.Client")
	}
}

func TestClientClonesEndpointOverrides(t *testing.T) {
	// Client documents itself as safe for concurrent use; sharing the caller's
	// map would make that untrue as soon as they mutated it.
	overrides := map[string]string{"trading": "first.example.com"}
	c, err := NewClient(Config{
		AppKey: "k", AppSecret: "s", Environment: Sandbox, EndpointOverrides: overrides,
	})
	if err != nil {
		t.Fatal(err)
	}

	overrides["trading"] = "second.example.com"

	got, err := c.cfg.host(serviceTrading)
	if err != nil {
		t.Fatal(err)
	}
	if got != "first.example.com" {
		t.Errorf("host = %q; the caller's later mutation leaked into the client", got)
	}
}

func TestClientRejectsEmptyBodyWhenAResultIsExpected(t *testing.T) {
	// Returning (false, nil) for an empty body is indistinguishable from the
	// server reporting false, which would silently disable token auth.
	c, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	if _, err := c.TokenCheckEnabled(context.Background()); err == nil {
		t.Fatal("an empty body must not decode as a zero value")
	}
}
