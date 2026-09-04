package webull

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sfreiberg/webull/internal/transport"
)

// TokenStatus is the lifecycle state of an access token.
type TokenStatus string

// Access token statuses.
const (
	// TokenPending means the token awaits verification through SMS and the
	// Webull app. The test environment skips verification.
	TokenPending TokenStatus = "PENDING"
	// TokenNormal means the token is valid.
	TokenNormal TokenStatus = "NORMAL"
	// TokenInvalid means the token is not usable.
	TokenInvalid TokenStatus = "INVALID"
	// TokenExpired means the token's lifetime has lapsed; create a new one.
	TokenExpired TokenStatus = "EXPIRED"
)

// AccessToken is a token for deployments that require token authentication
// in addition to the request signature — TokenCheckEnabled reports whether
// this deployment does. A token lives 15 days by default and is not
// refreshable; create a new one when it expires.
type AccessToken struct {
	// Token is the credential itself, sent as the x-access-token header by
	// a Client whose Config.AccessToken is set.
	Token string `json:"token"`
	// ExpiresAt is when the token becomes invalid.
	ExpiresAt time.Time `json:"-"`
	// Status is the token's lifecycle state. A newly created token starts
	// PENDING until verified through SMS and the Webull app; the test
	// environment skips verification.
	Status TokenStatus `json:"status"`
}

// UnmarshalJSON decodes the wire shape, whose expiry is epoch milliseconds
// that Webull may send as a number or a numeric string.
func (t *AccessToken) UnmarshalJSON(data []byte) error {
	var w struct {
		Token     string      `json:"token"`
		ExpiresAt flexMillis  `json:"expires_at"`
		Status    TokenStatus `json:"status"`
	}
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*t = AccessToken{Token: w.Token, ExpiresAt: time.Time(w.ExpiresAt), Status: w.Status}
	return nil
}

// flexMillis decodes an epoch-millisecond timestamp whether it arrives bare
// or quoted; null, an empty string, and zero decode as the zero time.
type flexMillis time.Time

func (m *flexMillis) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" || s == "0" {
		*m = flexMillis(time.Time{})
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("webull: %q is not an epoch-millisecond time", s)
	}
	*m = flexMillis(time.UnixMilli(n).UTC())
	return nil
}

// CreateAccessToken creates an access token for this app key. The token
// starts PENDING and must be verified through SMS and the Webull app before
// it reads NORMAL; the test environment skips verification. Pass the token
// to a new Client through Config.AccessToken.
func (c *Client) CreateAccessToken(ctx context.Context) (*AccessToken, error) {
	return c.postToken(ctx, "/auth/tokens/create", nil)
}

// CheckAccessToken reports a token's current status and expiry.
func (c *Client) CheckAccessToken(ctx context.Context, token string) (*AccessToken, error) {
	return c.postToken(ctx, "/auth/tokens/check", map[string]string{"token": token})
}

func (c *Client) postToken(ctx context.Context, path string, body any) (*AccessToken, error) {
	host, err := c.cfg.host(serviceTrading)
	if err != nil {
		return nil, err
	}
	var tok AccessToken
	if err := c.doer.Do(ctx, transport.Request{
		Method: "POST",
		Host:   host,
		Path:   path,
		Body:   body,
	}, &tok); err != nil {
		return nil, err
	}
	return &tok, nil
}
