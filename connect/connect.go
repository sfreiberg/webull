// Package connect implements Webull's Connect API: OAuth 2.0 access to
// other people's Webull accounts on their behalf, for platforms that let
// their users trade a linked Webull account without leaving the platform.
//
// It is not the path an individual takes to trade their own account — that
// is the root webull package, authenticated with an app key. Connect layers
// OAuth on top of the same signing: a Connect request is both signed and
// bearer-authenticated.
//
// # Flow
//
// The three-legged OAuth authorization-code flow:
//
//  1. Send the user to AuthorizationURL. After they consent, Webull
//     redirects to the registered RedirectURI with a one-time code.
//  2. Exchange that code for a Token with ExchangeCode.
//  3. Build a Client with the token; its Trade field reaches the same
//     trading and account operations as the root package, on the user's
//     account. The client refreshes the access token before it expires.
//
// Access tokens last 30 minutes and refresh tokens rotate on use, so a Token
// must be persisted (see TokenStore) for a session that outlives one access
// token.
//
// # Availability
//
// Connect credentials are issued by Webull to partner platforms, not
// self-service. This package is implemented against Webull's documentation;
// without credentials it has not been exercised against the live service.
package connect

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/internal/endpoints"
	"github.com/sfreiberg/webull/internal/wire"
)

// Time is a point in time in Webull's ISO 8601 form, shared with the other
// packages.
type Time = wire.Time

// oauthHosts resolves the Connect OAuth host per environment. The hostnames
// are shared constants with the root package's discovery notes; see
// internal/endpoints for why they are table entries.
var oauthHosts = map[webull.Environment]string{
	webull.Production: endpoints.ConnectProduction,
	webull.Sandbox:    endpoints.ConnectSandbox,
}

// Scope is an access scope requested in the authorization step.
type Scope string

// Scopes.
const (
	ScopeUser  Scope = "user"  // identity
	ScopeTrade Scope = "trade" // place and manage orders
	ScopeWR    Scope = "wr"    // write and read account data
)

// Token is an OAuth token pair with its lifetimes, as issued by the token
// endpoint. Persist it (TokenStore) so a session survives an access token's
// 30-minute lifetime; the refresh token rotates on every refresh.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	// Type is the authorization scheme, always "Bearer".
	Type string `json:"token_type"`
	// AccessExpiresIn and RefreshExpiresIn are lifetimes in seconds from
	// CreatedAt.
	AccessExpiresIn  int64 `json:"expires_in"`
	RefreshExpiresIn int64 `json:"rt_expires_in"`
	// CreatedAt is when the token was issued. When the endpoint omits it —
	// the standard OAuth response shape has no such field — the Authorizer
	// sets it to the time the response was received, so the expiry methods
	// are always usable.
	CreatedAt Time `json:"created_at"`
	// IdentityID is Webull's stable identifier for the authorizing user.
	IdentityID string `json:"identity_id"`
}

// UnmarshalJSON decodes the token endpoint's response, accepting the lifetime
// fields as either JSON strings or bare numbers. The OAuth 2.0 standard form
// is numeric and Webull's documentation does not pin the shape, so as with
// the SDK's other wire types, tolerance beats a hard failure on an otherwise
// successful grant.
func (t *Token) UnmarshalJSON(data []byte) error {
	var w struct {
		AccessToken      string    `json:"access_token"`
		RefreshToken     string    `json:"refresh_token"`
		Type             string    `json:"token_type"`
		AccessExpiresIn  flexInt64 `json:"expires_in"`
		RefreshExpiresIn flexInt64 `json:"rt_expires_in"`
		CreatedAt        Time      `json:"created_at"`
		IdentityID       string    `json:"identity_id"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*t = Token{
		AccessToken:      w.AccessToken,
		RefreshToken:     w.RefreshToken,
		Type:             w.Type,
		AccessExpiresIn:  int64(w.AccessExpiresIn),
		RefreshExpiresIn: int64(w.RefreshExpiresIn),
		CreatedAt:        w.CreatedAt,
		IdentityID:       w.IdentityID,
	}
	return nil
}

// flexInt64 decodes a JSON integer whether it arrives bare or quoted, and
// treats null or an empty string as zero.
type flexInt64 int64

func (f *flexInt64) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("connect: parsing %q as an integer: %w", s, err)
	}
	*f = flexInt64(n)
	return nil
}

// AccessExpiry is the moment the access token expires.
func (t Token) AccessExpiry() time.Time {
	return t.CreatedAt.Add(time.Duration(t.AccessExpiresIn) * time.Second)
}

// RefreshExpiry is the moment the refresh token expires; after this the user
// must authorize again.
func (t Token) RefreshExpiry() time.Time {
	return t.CreatedAt.Add(time.Duration(t.RefreshExpiresIn) * time.Second)
}

// accessValid reports whether the access token is usable at now, allowing a
// leeway so a token about to expire is refreshed first.
func (t Token) accessValid(now time.Time, leeway time.Duration) bool {
	return t.AccessToken != "" && now.Add(leeway).Before(t.AccessExpiry())
}
