package connect

import (
	"context"
	"sync"
)

// TokenStore persists one user's token across access-token lifetimes, so a
// session survives the 30-minute access-token expiry and process restarts.
// A Connect integration serves many users; give each Client a store scoped
// to its user (for example, a database-backed store keyed by the user).
//
// Load returns the stored token, or (nil, nil) when none is stored. Save
// records a token, replacing any previous one — the refresh token rotates,
// so the latest must overwrite the last.
type TokenStore interface {
	Load(ctx context.Context) (*Token, error)
	Save(ctx context.Context, tok *Token) error
}

// MemoryTokenStore keeps a token in memory. It is the default and is safe
// for concurrent use, but it does not survive a restart; supply a persistent
// TokenStore for anything longer-lived than one process.
type MemoryTokenStore struct {
	mu  sync.RWMutex
	tok *Token
}

// Load returns the stored token, or nil when none is stored.
func (m *MemoryTokenStore) Load(context.Context) (*Token, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tok, nil
}

// Save records the token.
func (m *MemoryTokenStore) Save(_ context.Context, tok *Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tok = tok
	return nil
}
