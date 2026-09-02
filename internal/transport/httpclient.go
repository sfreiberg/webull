package transport

import (
	"errors"
	"net/http"
	"time"
)

// DefaultTimeout applies to requests when no HTTP client is supplied.
const DefaultTimeout = 30 * time.Second

// ErrRedirectNotAllowed is returned when Webull responds with a redirect.
var ErrRedirectNotAllowed = errors.New("webull: refusing to follow a redirect")

// NewHTTPClient returns the client to use, with redirects refused. It is the
// one place that policy lives; every package that performs signed requests
// builds its client here.
//
// Redirects must not be followed. Go strips only Authorization and Cookie
// across hosts, so the signature headers — including the app key — would be
// forwarded verbatim to a redirect target, and Go permits an https-to-http
// downgrade. On a 307 or 308 the body is re-sent too, which for the Connect
// token endpoint would hand the client secret to the redirect target. The
// request could not succeed anyway, because the host is part of the signed
// canonical string.
//
// A caller-supplied client is shallow-copied rather than mutated, so the
// caller's own client is left alone. The copy shares its Transport, which is
// what keeps connection pooling intact.
func NewHTTPClient(base *http.Client) *http.Client {
	client := &http.Client{Timeout: DefaultTimeout}
	if base != nil {
		cp := *base
		client = &cp
	}
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return ErrRedirectNotAllowed
	}
	return client
}
