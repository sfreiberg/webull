package connect

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/internal/transport"
	"github.com/sfreiberg/webull/trade"
)

// Token-source errors.
var (
	// ErrNoToken is returned when a Client has no stored token: the user
	// has not been through the authorization flow, or the store is empty.
	ErrNoToken = errors.New("connect: no token; complete the authorization flow first")

	// ErrRefreshExpired is returned when the refresh token has itself
	// expired. The user must authorize again.
	ErrRefreshExpired = errors.New("connect: refresh token expired; the user must authorize again")
)

// refreshLeeway refreshes an access token this long before it expires, so a
// request is never sent with a token about to lapse.
const refreshLeeway = time.Minute

// Client accesses one user's Webull account through the Connect API. Its
// Trade field reaches the same trading and account operations as the root
// package, authenticated as that user. The access token is refreshed
// automatically before it expires; the rotating token pair is written back
// to the TokenStore.
//
// Build one with Authorizer.Client. It is safe for concurrent use.
type Client struct {
	// Trade covers accounts, balances, positions, activities, instrument
	// reference data and orders, on the authorized user's account.
	Trade *trade.Client

	src *tokenSource
}

// Client returns a Client for a user whose token has been obtained through
// the authorization flow. The token is saved to the configured TokenStore,
// and later refreshes rotate it there.
func (a *Authorizer) Client(ctx context.Context, tok *Token) (*Client, error) {
	if tok == nil || tok.AccessToken == "" {
		return nil, ErrNoToken
	}
	store := a.cfg.TokenStore
	if store == nil {
		store = &MemoryTokenStore{}
	}
	if err := store.Save(ctx, tok); err != nil {
		return nil, fmt.Errorf("connect: saving initial token: %w", err)
	}
	src := &tokenSource{auth: a, store: store, tok: tok, now: time.Now}

	doer := &transport.Doer{
		HTTPClient:  a.cfg.httpClient(),
		Signer:      a.doer.Signer,
		UserAgent:   a.doer.UserAgent,
		DecodeError: webull.ErrorDecoder(),
		Retry:       transport.DefaultRetryPolicy(),
		Authorizer:  src.bearer,
	}
	return &Client{
		Trade: trade.New(doer, a.host),
		src:   src,
	}, nil
}

// Token returns the current token, refreshing it first if it is near
// expiry. It is the way to read back the rotated token pair for external
// persistence beyond the TokenStore.
func (c *Client) Token(ctx context.Context) (*Token, error) {
	return c.src.valid(ctx)
}

// tokenSource holds a user's token and refreshes it under a lock, so
// concurrent requests never refresh the same token twice.
type tokenSource struct {
	auth  *Authorizer
	store TokenStore

	mu  sync.Mutex
	tok *Token
	// now is the clock; tests fix it.
	now func() time.Time
}

// bearer returns the Authorization header carrying a valid access token.
func (s *tokenSource) bearer(ctx context.Context) (map[string]string, error) {
	tok, err := s.valid(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{"Authorization": "Bearer " + tok.AccessToken}, nil
}

// valid returns a usable token, refreshing it when the access token is at or
// near expiry. It serializes refreshes so concurrent callers share one.
func (s *tokenSource) valid(ctx context.Context) (*Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tok == nil {
		loaded, err := s.store.Load(ctx)
		if err != nil {
			return nil, fmt.Errorf("connect: loading token: %w", err)
		}
		s.tok = loaded
	}
	if s.tok == nil {
		return nil, ErrNoToken
	}
	if s.tok.accessValid(s.now(), refreshLeeway) {
		return s.tok, nil
	}
	if !s.now().Before(s.tok.RefreshExpiry()) {
		return nil, ErrRefreshExpired
	}

	refreshed, err := s.auth.Refresh(ctx, s.tok.RefreshToken)
	if err != nil {
		return nil, err
	}
	s.tok = refreshed
	if err := s.store.Save(ctx, refreshed); err != nil {
		return nil, fmt.Errorf("connect: saving refreshed token: %w", err)
	}
	return refreshed, nil
}
