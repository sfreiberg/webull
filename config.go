package webull

import (
	"errors"
	"net/http"
	"time"
)

// DefaultTimeout applies to requests when Config.HTTPClient is nil.
const DefaultTimeout = 30 * time.Second

// Config describes how a Client connects to Webull.
//
// The zero value is not usable: AppKey, AppSecret and Environment are required.
type Config struct {
	// AppKey and AppSecret are the credentials issued by Webull. They are used
	// to sign every request and are never logged or included in errors.
	AppKey    string
	AppSecret string

	// Environment selects sandbox or production. There is no default: choosing
	// between simulated and real orders is not a decision this SDK will make on
	// a caller's behalf.
	Environment Environment

	// HTTPClient is used for all HTTP requests. If nil, a client with
	// DefaultTimeout is used. Supply your own to control transport, proxies or
	// connection pooling.
	HTTPClient *http.Client

	// EndpointOverrides replaces the host for a named service, for testing
	// against a local server or routing through a proxy. Keys are service
	// names: "trading", "marketdata", "streaming", "events", "connect".
	EndpointOverrides map[string]string

	// UserAgent is appended to the SDK's own User-Agent, identifying the
	// calling application. Optional.
	UserAgent string
}

// ErrMissingCredentials is returned when AppKey or AppSecret is empty.
var ErrMissingCredentials = errors.New("webull: AppKey and AppSecret are required")

// validate checks the configuration, returning an error that never contains
// credential material.
func (c *Config) validate() error {
	if c.AppKey == "" || c.AppSecret == "" {
		return ErrMissingCredentials
	}
	if !c.Environment.Valid() {
		return errors.New("webull: Environment must be webull.Sandbox or webull.Production")
	}
	return nil
}

// ErrRedirectNotAllowed is returned when Webull responds with a redirect.
var ErrRedirectNotAllowed = errors.New("webull: refusing to follow a redirect")

// httpClient returns the client to use, with redirects refused.
//
// Redirects must not be followed. Go strips only Authorization and Cookie
// across hosts, so the signature headers — including the app key — would be
// forwarded verbatim to a redirect target, and Go permits an https-to-http
// downgrade. The request could not succeed anyway, because the host is part of
// the signed canonical string.
//
// A caller-supplied client is shallow-copied rather than mutated, so the
// caller's own client is left alone. The copy shares its Transport, which is
// what keeps connection pooling intact.
func (c *Config) httpClient() *http.Client {
	client := &http.Client{Timeout: DefaultTimeout}
	if c.HTTPClient != nil {
		cp := *c.HTTPClient
		client = &cp
	}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return ErrRedirectNotAllowed
	}
	return client
}
