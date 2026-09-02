package connect

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/internal/signing"
	"github.com/sfreiberg/webull/internal/transport"
)

// Config holds the credentials and settings for a Connect integration.
type Config struct {
	// ClientID and ClientSecret are the OAuth credentials Webull issues to
	// the partner platform.
	ClientID     string
	ClientSecret string
	// AppKey and AppSecret sign every request; Connect requests are signed
	// as well as bearer-authenticated.
	AppKey    string
	AppSecret string
	// Environment selects sandbox or production. There is no default.
	Environment webull.Environment
	// RedirectURI is the callback Webull redirects to after authorization.
	// It must match one registered with Webull.
	RedirectURI string
	// Scopes are the access scopes requested; empty requests all of them.
	Scopes []string
	// Endpoint overrides the host derived from Environment, for testing
	// against a local server or routing through a proxy. Optional.
	Endpoint string
	// HTTPClient is used for all HTTP requests; nil uses a default with a
	// 30-second timeout. As with the root package, redirects are refused on
	// a copy of the supplied client, so signing material and the client
	// secret can never be forwarded to a redirect target.
	HTTPClient *http.Client
	// UserAgent identifies the calling application; optional.
	UserAgent string
}

func (c Config) validate() error {
	switch {
	case c.ClientID == "" || c.ClientSecret == "":
		return errors.New("connect: ClientID and ClientSecret are required")
	case c.AppKey == "" || c.AppSecret == "":
		return errors.New("connect: AppKey and AppSecret are required")
	case c.RedirectURI == "":
		return errors.New("connect: RedirectURI is required")
	}
	if _, ok := oauthHosts[c.Environment]; !ok {
		return fmt.Errorf("connect: unknown environment %q", c.Environment)
	}
	return nil
}

func (c Config) scopeString() string {
	if len(c.Scopes) == 0 {
		return strings.Join([]string{string(ScopeUser), string(ScopeTrade), string(ScopeWR)}, ":")
	}
	return strings.Join(c.Scopes, ":")
}

// host returns the configured override, or the environment's host.
func (c Config) host() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return oauthHosts[c.Environment]
}

// Authorizer builds authorization URLs and exchanges codes for tokens. It
// does not hold a user's token; a Client does. It is the entry point to the
// OAuth flow and is safe for concurrent use.
type Authorizer struct {
	cfg  Config
	host string
	doer *transport.Doer
	// now is the clock, used to default a token's CreatedAt; tests fix it.
	now func() time.Time
}

// NewAuthorizer returns an Authorizer for the given configuration.
func NewAuthorizer(cfg Config) (*Authorizer, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	userAgent := webull.UserAgent()
	if cfg.UserAgent != "" {
		userAgent = cfg.UserAgent + " " + userAgent
	}
	return &Authorizer{
		cfg:  cfg,
		host: cfg.host(),
		doer: &transport.Doer{
			HTTPClient:  transport.NewHTTPClient(cfg.HTTPClient),
			Signer:      signing.New(cfg.AppKey, cfg.AppSecret),
			UserAgent:   userAgent,
			DecodeError: transport.APIErrorDecoder,
			Retry:       transport.DefaultRetryPolicy(),
		},
		now: time.Now,
	}, nil
}

// AuthorizationURL returns the URL to send a user to so they can authorize
// the application. After they consent, Webull redirects to the configured
// RedirectURI with a one-time code and the state echoed back.
//
// state must be an unguessable value the caller generates and later checks
// against the redirect, to defend against cross-site request forgery.
func (a *Authorizer) AuthorizationURL(state string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", a.cfg.ClientID)
	q.Set("scope", a.cfg.scopeString())
	q.Set("state", state)
	q.Set("redirect_uri", a.cfg.RedirectURI)
	return (&url.URL{Scheme: "https", Host: a.host, Path: "/oauth2/auth-codes/get", RawQuery: q.Encode()}).String()
}

// ExchangeCode exchanges an authorization code from the redirect for a
// token. The code is single-use and expires 60 seconds after it is issued.
func (a *Authorizer) ExchangeCode(ctx context.Context, code string) (*Token, error) {
	return a.token(ctx, url.Values{
		"grant_type": {"authorization_code"},
		"code":       {code},
	})
}

// Refresh exchanges a refresh token for a new token. Webull rotates the
// refresh token on every refresh, so the returned Token carries a new
// refresh token that supersedes the old one.
func (a *Authorizer) Refresh(ctx context.Context, refreshToken string) (*Token, error) {
	return a.token(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	})
}

// token posts to the token endpoint with the shared fields added.
func (a *Authorizer) token(ctx context.Context, form url.Values) (*Token, error) {
	form.Set("client_id", a.cfg.ClientID)
	form.Set("client_secret", a.cfg.ClientSecret)
	var tok Token
	err := a.doer.Do(ctx, transport.Request{
		Method: "POST",
		Host:   a.host,
		Path:   "/oauth2/tokens/create",
		Form:   form,
	}, &tok)
	if err != nil {
		return nil, err
	}
	if tok.CreatedAt.IsZero() {
		// The standard OAuth response shape has no created_at. Without a
		// value the expiries would sit in year 1 and a seconds-old token
		// would read as terminally expired, so receipt time stands in.
		tok.CreatedAt = Time{Time: a.now()}
	}
	return &tok, nil
}
