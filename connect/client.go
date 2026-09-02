package connect

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

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
// automatically before it expires, and the rotating pair is saved back to
// the client's TokenStore.
//
// Build one with Authorizer.Client or Authorizer.ClientFromStore. It is safe
// for concurrent use. Constructing a Client performs no I/O.
type Client struct {
	// Trade covers accounts, balances, positions, activities, instrument
	// reference data and orders, on the authorized user's account.
	Trade *trade.Client

	src *tokenSource
}

// Client returns a Client for a user from a token just obtained through the
// authorization flow. The pair is held in memory: refreshes rotate it there,
// and Token reads it back for persistence. Use ClientFromStore when the pair
// should live in a TokenStore of your own instead.
func (a *Authorizer) Client(tok *Token) (*Client, error) {
	if tok == nil || tok.AccessToken == "" {
		return nil, ErrNoToken
	}
	return a.ClientFromStore(NewMemoryTokenStore(tok))
}

// ClientFromStore returns a Client whose token pair lives in store, which
// holds one user's pair and is the source of truth: the pair is loaded on
// first use, and refreshes save the rotated pair back. Nothing is written
// until a refresh happens, so a pair already in the store — possibly newer
// than anything the caller holds — is never overwritten with a stale one.
//
// A platform serving many users keeps one store per connected user.
func (a *Authorizer) ClientFromStore(store TokenStore) (*Client, error) {
	if store == nil {
		return nil, errors.New("connect: a TokenStore is required")
	}
	src := &tokenSource{auth: a, store: store, now: time.Now}

	doer := &transport.Doer{
		HTTPClient:  transport.NewHTTPClient(a.cfg.HTTPClient),
		Signer:      a.doer.Signer,
		UserAgent:   a.doer.UserAgent,
		DecodeError: transport.APIErrorDecoder,
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

// tokenSource holds a user's token and collapses concurrent refreshes into
// one call whose outcome — the new token, or the failure — every waiter
// shares.
type tokenSource struct {
	auth  *Authorizer
	store TokenStore

	mu       sync.Mutex
	tok      *Token
	inflight *refreshCall
	// now is the clock; tests fix it.
	now func() time.Time
}

// refreshCall is one in-flight refresh. Concurrent callers wait on done and
// share the result: a success hands them all the new token, and a failure
// fails them all at once instead of each in turn re-driving a refresh that
// just failed against the token endpoint.
type refreshCall struct {
	done chan struct{}
	tok  *Token
	err  error
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
// near expiry.
func (s *tokenSource) valid(ctx context.Context) (*Token, error) {
	s.mu.Lock()
	if s.tok == nil {
		loaded, err := s.store.Load(ctx)
		if err != nil {
			s.mu.Unlock()
			return nil, fmt.Errorf("connect: loading token: %w", err)
		}
		s.tok = loaded
	}
	if s.tok == nil {
		s.mu.Unlock()
		return nil, ErrNoToken
	}
	if s.tok.accessValid(s.now(), refreshLeeway) {
		tok := s.tok
		s.mu.Unlock()
		return tok, nil
	}
	if !s.now().Before(s.tok.RefreshExpiry()) {
		s.mu.Unlock()
		return nil, ErrRefreshExpired
	}

	if s.inflight != nil {
		call := s.inflight
		s.mu.Unlock()
		select {
		case <-call.done:
			return call.tok, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	call := &refreshCall{done: make(chan struct{})}
	s.inflight = call
	refreshToken := s.tok.RefreshToken
	s.mu.Unlock()

	// The network calls run outside the lock, so waiters can still honour
	// their own contexts while this refresh is in flight.
	refreshed, err := s.auth.Refresh(ctx, refreshToken)
	if err == nil {
		if saveErr := s.store.Save(ctx, refreshed); saveErr != nil {
			err = fmt.Errorf("connect: saving refreshed token: %w", saveErr)
		}
	}

	s.mu.Lock()
	if refreshed != nil {
		// Kept even when saving failed: the rotation consumed the old
		// refresh token, so this pair is the only valid one.
		s.tok = refreshed
	}
	s.inflight = nil
	s.mu.Unlock()

	if err != nil {
		call.err = err
	} else {
		call.tok = refreshed
	}
	close(call.done)
	return call.tok, call.err
}
