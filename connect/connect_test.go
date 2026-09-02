package connect

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sfreiberg/webull"
)

// fakeServer stands in for Webull's OAuth and trading endpoints.
type fakeServer struct {
	tokenCalls atomic.Int32
	tradeAuth  atomic.Value // last Authorization header seen on a trading call
	lastForm   url.Values
	nextToken  func(call int, form url.Values) Token
	mu         sync.Mutex
}

func (f *fakeServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth2/tokens/create":
			body, _ := io.ReadAll(r.Body)
			form, _ := url.ParseQuery(string(body))
			f.mu.Lock()
			f.lastForm = form
			f.mu.Unlock()
			call := int(f.tokenCalls.Add(1))
			if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				http.Error(w, `{"error_code":"BAD","message":"wrong content type"}`, http.StatusExpectationFailed)
				return
			}
			if r.Header.Get("x-signature") == "" {
				http.Error(w, `{"error_code":"BAD","message":"unsigned"}`, http.StatusUnauthorized)
				return
			}
			tok := f.nextToken(call, form)
			writeToken(w, tok)
		case "/trading/accounts/list":
			f.tradeAuth.Store(r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`[{"account_id":"ACCT-1","account_class":"INDIVIDUAL_MARGIN"}]`))
		default:
			http.Error(w, `{"error_code":"NOT_FOUND","message":"`+r.URL.Path+`"}`, http.StatusNotFound)
		}
	})
}

// writeToken serialises a Token as the wire form: lifetimes are strings.
func writeToken(w http.ResponseWriter, tok Token) {
	created := tok.CreatedAt.UTC().Format("2006-01-02T15:04:05.000-0700")
	_, _ = w.Write([]byte(`{` +
		`"access_token":"` + tok.AccessToken + `",` +
		`"refresh_token":"` + tok.RefreshToken + `",` +
		`"token_type":"Bearer",` +
		`"expires_in":"` + itoa(tok.AccessExpiresIn) + `",` +
		`"rt_expires_in":"` + itoa(tok.RefreshExpiresIn) + `",` +
		`"created_at":"` + created + `",` +
		`"identity_id":"` + tok.IdentityID + `"}`))
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func newFixture(t *testing.T, f *fakeServer) *Authorizer {
	t.Helper()
	srv := httptest.NewTLSServer(f.handler())
	t.Cleanup(srv.Close)
	a, err := NewAuthorizer(Config{
		ClientID: "cid", ClientSecret: "csec", AppKey: "k", AppSecret: "s",
		Environment: webull.Sandbox, RedirectURI: "https://app.example/callback",
		HTTPClient: srv.Client(),
		Endpoint:   strings.TrimPrefix(srv.URL, "https://"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func mkToken(access, refresh string, created time.Time) Token {
	return Token{
		AccessToken: access, RefreshToken: refresh, Type: "Bearer",
		AccessExpiresIn: 1800, RefreshExpiresIn: 1296000,
		CreatedAt: Time{Time: created}, IdentityID: "id-1",
	}
}

func TestAuthorizationURL(t *testing.T) {
	a, err := NewAuthorizer(Config{
		ClientID: "my-client", ClientSecret: "secret", AppKey: "k", AppSecret: "s",
		Environment: webull.Sandbox, RedirectURI: "https://app.example/cb",
		Scopes: []string{string(ScopeUser), string(ScopeTrade)},
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := url.Parse(a.AuthorizationURL("xyz-state"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Host != "oauth-open-api.sandbox.webull.com" || got.Path != "/oauth2/auth-codes/get" {
		t.Errorf("url = %s", got)
	}
	q := got.Query()
	for k, want := range map[string]string{
		"response_type": "code", "client_id": "my-client", "scope": "user:trade",
		"state": "xyz-state", "redirect_uri": "https://app.example/cb",
	} {
		if q.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, q.Get(k), want)
		}
	}
}

func TestAuthorizationURLDefaultsAllScopes(t *testing.T) {
	a, _ := NewAuthorizer(Config{
		ClientID: "c", ClientSecret: "s", AppKey: "k", AppSecret: "s",
		Environment: webull.Production, RedirectURI: "https://x",
	})
	u, _ := url.Parse(a.AuthorizationURL("st"))
	if u.Host != "us-oauth-open-api.webull.com" {
		t.Errorf("production host = %s", u.Host)
	}
	if got := u.Query().Get("scope"); got != "user:trade:wr" {
		t.Errorf("default scope = %q", got)
	}
}

func TestExchangeCodeSignsAndDecodes(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	f := &fakeServer{nextToken: func(_ int, _ url.Values) Token {
		return mkToken("access-1", "refresh-1", now)
	}}
	a := newFixture(t, f)

	tok, err := a.ExchangeCode(context.Background(), "the-code")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "access-1" || tok.RefreshToken != "refresh-1" || tok.IdentityID != "id-1" {
		t.Errorf("token = %+v", tok)
	}
	if tok.AccessExpiresIn != 1800 || !tok.AccessExpiry().Equal(now.Add(30*time.Minute)) {
		t.Errorf("expiry = %v", tok.AccessExpiry())
	}
	f.mu.Lock()
	form := f.lastForm
	f.mu.Unlock()
	if form.Get("grant_type") != "authorization_code" || form.Get("code") != "the-code" ||
		form.Get("client_id") != "cid" || form.Get("client_secret") != "csec" {
		t.Errorf("form = %v", form)
	}
}

func TestRefreshRotatesToken(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	f := &fakeServer{nextToken: func(_ int, _ url.Values) Token {
		return mkToken("access-2", "refresh-2", now)
	}}
	a := newFixture(t, f)
	tok, err := a.Refresh(context.Background(), "old-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if tok.RefreshToken != "refresh-2" {
		t.Errorf("rotated refresh = %q", tok.RefreshToken)
	}
	f.mu.Lock()
	if f.lastForm.Get("grant_type") != "refresh_token" || f.lastForm.Get("refresh_token") != "old-refresh" {
		t.Errorf("form = %v", f.lastForm)
	}
	f.mu.Unlock()
}

func TestClientCarriesBearerToken(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	f := &fakeServer{nextToken: func(int, url.Values) Token { return mkToken("A", "R", now) }}
	a := newFixture(t, f)

	c, err := a.Client(ptr(mkToken("access-live", "refresh-live", now)))
	if err != nil {
		t.Fatal(err)
	}
	// Freeze the clock so the seeded token is considered valid.
	c.src.now = func() time.Time { return now }

	accts, err := c.Trade.Accounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(accts) != 1 || accts[0].AccountID != "ACCT-1" {
		t.Errorf("accounts = %+v", accts)
	}
	if got := f.tradeAuth.Load(); got != "Bearer access-live" {
		t.Errorf("Authorization = %q, want Bearer access-live", got)
	}
	if f.tokenCalls.Load() != 0 {
		t.Errorf("a valid token must not be refreshed: %d token calls", f.tokenCalls.Load())
	}
}

func ptr[T any](v T) *T { return &v }

func TestNearExpiryTriggersRefresh(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	// Seeded token was created 29m30s ago, so it is inside the 1m leeway.
	seeded := mkToken("stale-access", "good-refresh", now.Add(-29*time.Minute-30*time.Second))
	f := &fakeServer{nextToken: func(_ int, form url.Values) Token {
		if form.Get("refresh_token") != "good-refresh" {
			t.Errorf("refreshed with %q", form.Get("refresh_token"))
		}
		return mkToken("fresh-access", "rotated-refresh", now)
	}}
	a := newFixture(t, f)
	c, err := a.Client(&seeded)
	if err != nil {
		t.Fatal(err)
	}
	c.src.now = func() time.Time { return now }

	if _, err := c.Trade.Accounts(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.tradeAuth.Load() != "Bearer fresh-access" {
		t.Errorf("near-expiry token was not refreshed: %v", f.tradeAuth.Load())
	}
	if f.tokenCalls.Load() != 1 {
		t.Errorf("token calls = %d, want 1", f.tokenCalls.Load())
	}
	// The rotated pair is persisted for reuse.
	tok, _ := c.Token(context.Background())
	if tok.RefreshToken != "rotated-refresh" {
		t.Errorf("store did not keep the rotated refresh token: %+v", tok)
	}
}

func TestExpiredRefreshTokenIsTerminal(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	// Both access and refresh are long expired.
	seeded := mkToken("old", "old-refresh", now.Add(-30*24*time.Hour))
	a := newFixture(t, &fakeServer{nextToken: func(int, url.Values) Token { return mkToken("x", "y", now) }})
	c, err := a.Client(&seeded)
	if err != nil {
		t.Fatal(err)
	}
	c.src.now = func() time.Time { return now }
	if _, err := c.Token(context.Background()); !errors.Is(err, ErrRefreshExpired) {
		t.Errorf("err = %v, want ErrRefreshExpired", err)
	}
}

func TestConcurrentRefreshHappensOnce(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	seeded := mkToken("stale", "r0", now.Add(-40*time.Minute)) // access expired
	f := &fakeServer{nextToken: func(call int, _ url.Values) Token {
		return mkToken("fresh", "r1", now)
	}}
	a := newFixture(t, f)
	c, err := a.Client(&seeded)
	if err != nil {
		t.Fatal(err)
	}
	c.src.now = func() time.Time { return now }

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.Token(context.Background()); err != nil {
				t.Errorf("Token: %v", err)
			}
		}()
	}
	wg.Wait()
	if n := f.tokenCalls.Load(); n != 1 {
		t.Errorf("token endpoint called %d times; concurrent refresh must collapse to one", n)
	}
}

func TestClientFromStoreLoadsLazily(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	store := NewMemoryTokenStore(ptr(mkToken("stored-access", "stored-refresh", now)))

	f := &fakeServer{nextToken: func(int, url.Values) Token { return mkToken("x", "y", now) }}
	a := newFixture(t, f)
	c, err := a.ClientFromStore(store)
	if err != nil {
		t.Fatal(err)
	}
	c.src.now = func() time.Time { return now }
	tok, err := c.Token(context.Background())
	if err != nil || tok.AccessToken != "stored-access" {
		t.Errorf("did not load from store: %+v %v", tok, err)
	}
}

// countingStore records writes, to prove construction never writes.
type countingStore struct {
	MemoryTokenStore
	saves atomic.Int32
}

func (c *countingStore) Save(ctx context.Context, tok *Token) error {
	c.saves.Add(1)
	return c.MemoryTokenStore.Save(ctx, tok)
}

func TestClientFromStoreNeverClobbersTheStore(t *testing.T) {
	// The store may hold a pair newer than anything the caller has cached —
	// refresh tokens rotate — so nothing may be written until a refresh
	// produces a genuinely newer pair.
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	store := &countingStore{}
	_ = store.MemoryTokenStore.Save(context.Background(), ptr(mkToken("newer-access", "r1", now)))

	a := newFixture(t, &fakeServer{nextToken: func(int, url.Values) Token { return mkToken("x", "y", now) }})
	c, err := a.ClientFromStore(store)
	if err != nil {
		t.Fatal(err)
	}
	c.src.now = func() time.Time { return now }
	tok, err := c.Token(context.Background())
	if err != nil || tok.RefreshToken != "r1" {
		t.Fatalf("tok = %+v, %v", tok, err)
	}
	if n := store.saves.Load(); n != 0 {
		t.Errorf("store written %d times before any refresh; the stored pair must win", n)
	}
}

func TestClientFromStoreRequiresAStore(t *testing.T) {
	a := newFixture(t, &fakeServer{nextToken: func(int, url.Values) Token { return Token{} }})
	if _, err := a.ClientFromStore(nil); err == nil {
		t.Error("a nil store must be rejected")
	}
}

func TestConfigValidation(t *testing.T) {
	base := Config{ClientID: "c", ClientSecret: "s", AppKey: "k", AppSecret: "s", Environment: webull.Sandbox, RedirectURI: "https://x"}
	if _, err := NewAuthorizer(base); err != nil {
		t.Fatalf("valid config rejected: %v", err)
	}
	for name, mut := range map[string]func(*Config){
		"no client":   func(c *Config) { c.ClientID = "" },
		"no app key":  func(c *Config) { c.AppKey = "" },
		"no redirect": func(c *Config) { c.RedirectURI = "" },
		"bad env":     func(c *Config) { c.Environment = "staging" },
	} {
		cfg := base
		mut(&cfg)
		if _, err := NewAuthorizer(cfg); err == nil {
			t.Errorf("%s: expected a validation error", name)
		}
	}
}

func TestClientRejectsEmptyToken(t *testing.T) {
	a := newFixture(t, &fakeServer{nextToken: func(int, url.Values) Token { return Token{} }})
	if _, err := a.Client(nil); !errors.Is(err, ErrNoToken) {
		t.Errorf("nil token: err = %v, want ErrNoToken", err)
	}
	if _, err := a.Client(&Token{}); !errors.Is(err, ErrNoToken) {
		t.Errorf("empty token: err = %v, want ErrNoToken", err)
	}
}

func TestTokenEndpointErrorIsAPIError(t *testing.T) {
	// A server that rejects the exchange with a Webull error shape.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusExpectationFailed)
		_, _ = w.Write([]byte(`{"error_code":"OPENAPI_PARAM_ERR","message":"invalid code"}`))
	}))
	t.Cleanup(srv.Close)
	a, _ := NewAuthorizer(Config{
		ClientID: "c", ClientSecret: "s", AppKey: "k", AppSecret: "s",
		Environment: webull.Sandbox, RedirectURI: "https://x", HTTPClient: srv.Client(),
		Endpoint: strings.TrimPrefix(srv.URL, "https://"),
	})
	_, err := a.ExchangeCode(context.Background(), "bad")
	var apiErr *webull.APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "OPENAPI_PARAM_ERR" {
		t.Errorf("err = %v, want an *APIError with the code", err)
	}
}

func TestRefreshFailureSurfacesThroughTradeCall(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	seeded := mkToken("expired", "r", now.Add(-40*time.Minute)) // needs refresh
	f := &fakeServer{nextToken: func(int, url.Values) Token { return Token{} }}
	// Override: the token endpoint fails.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth2/tokens/create" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error_code":"UNAUTHORIZED","message":"nope"}`))
			return
		}
		f.handler().ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	a, _ := NewAuthorizer(Config{
		ClientID: "c", ClientSecret: "s", AppKey: "k", AppSecret: "s",
		Environment: webull.Sandbox, RedirectURI: "https://x", HTTPClient: srv.Client(),
		Endpoint: strings.TrimPrefix(srv.URL, "https://"),
	})
	c, err := a.Client(&seeded)
	if err != nil {
		t.Fatal(err)
	}
	c.src.now = func() time.Time { return now }
	// The trade call cannot authorize because the refresh fails.
	if _, err := c.Trade.Accounts(context.Background()); err == nil {
		t.Error("a failed refresh must fail the trade call")
	}
}

// errStore fails on Load, to exercise the error path.
type errStore struct{}

func (errStore) Load(context.Context) (*Token, error) { return nil, errors.New("db down") }
func (errStore) Save(context.Context, *Token) error   { return nil }

func TestStoreLoadErrorSurfaces(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	a := newFixture(t, &fakeServer{nextToken: func(int, url.Values) Token { return Token{} }})
	c, err := a.Client(ptr(mkToken("a", "r", now)))
	if err != nil {
		t.Fatal(err)
	}
	c.src.store = errStore{}
	c.src.tok = nil // force a load
	if _, err := c.Token(context.Background()); err == nil || !strings.Contains(err.Error(), "loading token") {
		t.Errorf("err = %v, want a load error", err)
	}
}

func TestNewAuthorizerAppendsUserAgent(t *testing.T) {
	a, err := NewAuthorizer(Config{
		ClientID: "c", ClientSecret: "s", AppKey: "k", AppSecret: "s",
		Environment: webull.Sandbox, RedirectURI: "https://x", UserAgent: "acme/2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a.doer.UserAgent, "acme/2 ") {
		t.Errorf("user agent = %q", a.doer.UserAgent)
	}
}

func TestTokenDecodesQuotedAndNumericLifetimes(t *testing.T) {
	// Observed Webull responses quote their numbers; the OAuth 2.0 standard
	// form is numeric. Both must decode.
	for name, body := range map[string]string{
		"quoted":  `{"access_token":"a","refresh_token":"r","expires_in":"1800","rt_expires_in":"1296000"}`,
		"numeric": `{"access_token":"a","refresh_token":"r","expires_in":1800,"rt_expires_in":1296000}`,
	} {
		var tok Token
		if err := json.Unmarshal([]byte(body), &tok); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if tok.AccessExpiresIn != 1800 || tok.RefreshExpiresIn != 1296000 {
			t.Errorf("%s: lifetimes = %d, %d", name, tok.AccessExpiresIn, tok.RefreshExpiresIn)
		}
	}
	var tok Token
	if err := json.Unmarshal([]byte(`{"expires_in":"soon"}`), &tok); err == nil {
		t.Error("a non-numeric lifetime must be an error, not a silent zero")
	}
}

func TestMissingCreatedAtDefaultsToReceiptTime(t *testing.T) {
	// The standard OAuth response has no created_at; without a fallback the
	// expiries would sit in year 1 and a fresh token would read as expired.
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"a","refresh_token":"r","token_type":"Bearer",` +
			`"expires_in":1800,"rt_expires_in":1296000,"identity_id":"id"}`))
	}))
	t.Cleanup(srv.Close)
	a, err := NewAuthorizer(Config{
		ClientID: "c", ClientSecret: "s", AppKey: "k", AppSecret: "s",
		Environment: webull.Sandbox, RedirectURI: "https://x", HTTPClient: srv.Client(),
		Endpoint: strings.TrimPrefix(srv.URL, "https://"),
	})
	if err != nil {
		t.Fatal(err)
	}
	a.now = func() time.Time { return now }
	tok, err := a.ExchangeCode(context.Background(), "code")
	if err != nil {
		t.Fatal(err)
	}
	if !tok.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want the receipt time", tok.CreatedAt)
	}
	if !tok.AccessExpiry().Equal(now.Add(30 * time.Minute)) {
		t.Errorf("AccessExpiry = %v", tok.AccessExpiry())
	}
	if !tok.accessValid(now, refreshLeeway) {
		t.Error("a fresh token must be valid at receipt")
	}
}

func TestFailedRefreshIsSharedByConcurrentCallers(t *testing.T) {
	// When a refresh fails, every caller queued behind it must share that
	// failure rather than each re-driving its own doomed refresh against a
	// token endpoint that just said no.
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	entered := make(chan struct{})
	release := make(chan struct{})
	var tokenCalls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tokenCalls.Add(1) == 1 {
			close(entered)
		}
		<-release
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error_code":"UNAUTHORIZED","message":"nope"}`))
	}))
	t.Cleanup(srv.Close)
	a, err := NewAuthorizer(Config{
		ClientID: "c", ClientSecret: "s", AppKey: "k", AppSecret: "s",
		Environment: webull.Sandbox, RedirectURI: "https://x", HTTPClient: srv.Client(),
		Endpoint: strings.TrimPrefix(srv.URL, "https://"),
	})
	if err != nil {
		t.Fatal(err)
	}
	seeded := mkToken("expired", "r0", now.Add(-40*time.Minute))
	c, err := a.Client(&seeded)
	if err != nil {
		t.Fatal(err)
	}
	c.src.now = func() time.Time { return now }

	var wg sync.WaitGroup
	errs := make([]error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = c.Token(context.Background())
		}(i)
	}
	// Hold the one in-flight refresh open until every other goroutine has had
	// ample time to queue behind it, then let it fail.
	<-entered
	time.Sleep(250 * time.Millisecond)
	close(release)
	wg.Wait()

	for i, err := range errs {
		if err == nil {
			t.Errorf("caller %d: expected the shared refresh failure", i)
		}
	}
	if n := tokenCalls.Load(); n != 1 {
		t.Errorf("token endpoint called %d times; a failed refresh must be shared, not serially retried", n)
	}
}

func TestWaiterContextCancelsIndependently(t *testing.T) {
	// A waiter whose own context expires must not be held hostage by the
	// in-flight refresh.
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	entered := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(entered) })
		<-release
		writeToken(w, mkToken("fresh", "r1", now))
	}))
	t.Cleanup(srv.Close)
	a, err := NewAuthorizer(Config{
		ClientID: "c", ClientSecret: "s", AppKey: "k", AppSecret: "s",
		Environment: webull.Sandbox, RedirectURI: "https://x", HTTPClient: srv.Client(),
		Endpoint: strings.TrimPrefix(srv.URL, "https://"),
	})
	if err != nil {
		t.Fatal(err)
	}
	seeded := mkToken("expired", "r0", now.Add(-40*time.Minute))
	c, err := a.Client(&seeded)
	if err != nil {
		t.Fatal(err)
	}
	c.src.now = func() time.Time { return now }

	// Leader blocks in the refresh.
	leaderDone := make(chan error, 1)
	go func() {
		_, err := c.Token(context.Background())
		leaderDone <- err
	}()
	<-entered

	// A waiter with an already-cancelled context returns at once.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Token(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("waiter err = %v, want context.Canceled", err)
	}

	close(release)
	if err := <-leaderDone; err != nil {
		t.Errorf("leader: %v", err)
	}
}

func TestFlexInt64TreatsAbsentShapesAsZero(t *testing.T) {
	var tok Token
	if err := json.Unmarshal([]byte(`{"access_token":"a","expires_in":null,"rt_expires_in":""}`), &tok); err != nil {
		t.Fatal(err)
	}
	if tok.AccessExpiresIn != 0 || tok.RefreshExpiresIn != 0 {
		t.Errorf("lifetimes = %d, %d, want zeros for null and empty", tok.AccessExpiresIn, tok.RefreshExpiresIn)
	}
}

func TestClientFromStoreEmptyStoreIsErrNoToken(t *testing.T) {
	a := newFixture(t, &fakeServer{nextToken: func(int, url.Values) Token { return Token{} }})
	c, err := a.ClientFromStore(&MemoryTokenStore{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Token(context.Background()); !errors.Is(err, ErrNoToken) {
		t.Errorf("err = %v, want ErrNoToken", err)
	}
}

// failingSaveStore loads normally but cannot persist.
type failingSaveStore struct {
	MemoryTokenStore
}

func (f *failingSaveStore) Save(context.Context, *Token) error {
	return errors.New("db down")
}

func TestSaveFailureAfterRefreshKeepsTheRotatedPair(t *testing.T) {
	// The rotation consumed the old refresh token, so even when the store
	// cannot persist the new pair, the client must keep it in memory: the
	// request fails loudly, but the session is not lost.
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	f := &fakeServer{nextToken: func(int, url.Values) Token { return mkToken("fresh", "r1", now) }}
	a := newFixture(t, f)
	store := &failingSaveStore{}
	_ = store.MemoryTokenStore.Save(context.Background(), ptr(mkToken("expired", "r0", now.Add(-40*time.Minute))))

	c, err := a.ClientFromStore(store)
	if err != nil {
		t.Fatal(err)
	}
	c.src.now = func() time.Time { return now }

	if _, err := c.Token(context.Background()); err == nil || !strings.Contains(err.Error(), "saving refreshed token") {
		t.Fatalf("err = %v, want a save error", err)
	}
	// The rotated pair survives in memory: the next call succeeds without
	// another refresh.
	tok, err := c.Token(context.Background())
	if err != nil || tok.RefreshToken != "r1" {
		t.Errorf("tok = %+v, %v; want the rotated pair retained", tok, err)
	}
	if n := f.tokenCalls.Load(); n != 1 {
		t.Errorf("token endpoint called %d times, want 1", n)
	}
}
