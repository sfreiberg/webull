package webull

import (
	"errors"
	"net/http"
	"time"

	"github.com/sfreiberg/webull/internal/transport"
)

// DefaultTimeout applies to requests when Config.HTTPClient is nil. It is
// restated here rather than referencing transport.DefaultTimeout so godoc
// shows the value; a test asserts the two stay equal.
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
	// names: "trading", "marketdata", "streaming", "events". Each is
	// resolved independently: trading and market data share a host by
	// default, but overriding one does not override the other. The Connect
	// API is configured through connect.Config, which has its own Endpoint
	// override.
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
// Redirects are refused because Go forwards the signature headers — including
// the app key — to the redirect target; see transport.NewHTTPClient.
var ErrRedirectNotAllowed = transport.ErrRedirectNotAllowed

// httpClient returns the client to use, with redirects refused. The policy
// lives in transport.NewHTTPClient, shared with the connect package.
func (c *Config) httpClient() *http.Client {
	return transport.NewHTTPClient(c.HTTPClient)
}
