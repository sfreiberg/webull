package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sfreiberg/webull/internal/query"
)

// ErrWatchlistFailed is returned when Webull answers a watchlist mutation
// with HTTP 200 but reports it unsuccessful.
var ErrWatchlistFailed = errors.New("marketdata: webull reported the watchlist operation unsuccessful")

// Watchlist is one of the account's watchlists.
type Watchlist struct {
	ID   string `json:"watchlist_id"`
	Name string `json:"name"`
	// Sort orders watchlists for display.
	Sort       int  `json:"sort"`
	CreateTime Time `json:"create_time"`
	UpdateTime Time `json:"update_time"`
}

// Watchlists returns the account's watchlists.
func (c *Client) Watchlists(ctx context.Context) ([]Watchlist, error) {
	var out []Watchlist
	if err := c.get(ctx, "/market-data/watchlists/list", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateWatchlist creates a watchlist and returns its ID. Sort orders it
// for display; zero lets Webull assign one.
func (c *Client) CreateWatchlist(ctx context.Context, name string, sort int) (string, error) {
	body := struct {
		Name string `json:"name"`
		Sort int    `json:"sort,omitzero"`
	}{Name: name, Sort: sort}
	var out struct {
		ID string `json:"watchlist_id"`
	}
	if err := c.post(ctx, "/market-data/watchlists/create", body, &out); err != nil {
		return "", err
	}
	return out.ID, nil
}

// successBody decodes a watchlist mutation receipt in either of the shapes
// Webull uses: the {"success": bool} envelope its documentation shows, or
// the bare JSON bool the sandbox returns. Any other shape is a decode
// error, so an unrecognised receipt fails loudly rather than being
// misread as a failed mutation.
type successBody struct {
	Success bool
}

func (s *successBody) UnmarshalJSON(b []byte) error {
	switch trimmed := strings.TrimSpace(string(b)); trimmed {
	case "true", "false":
		s.Success = trimmed == "true"
		return nil
	}
	var wrapped struct {
		Success *bool `json:"success"`
	}
	if err := json.Unmarshal(b, &wrapped); err != nil {
		return err
	}
	if wrapped.Success == nil {
		return fmt.Errorf("marketdata: unrecognised watchlist receipt %s", b)
	}
	s.Success = *wrapped.Success
	return nil
}

func (s successBody) err(op string) error {
	if !s.Success {
		return fmt.Errorf("%w: %s", ErrWatchlistFailed, op)
	}
	return nil
}

// UpdateWatchlist renames or reorders a watchlist. An empty name or a zero
// sort leaves that attribute unchanged.
func (c *Client) UpdateWatchlist(ctx context.Context, id, name string, sort int) error {
	body := struct {
		ID   string `json:"watchlist_id"`
		Name string `json:"name,omitzero"`
		Sort int    `json:"sort,omitzero"`
	}{ID: id, Name: name, Sort: sort}
	var out successBody
	if err := c.post(ctx, "/market-data/watchlists/update", body, &out); err != nil {
		return err
	}
	return out.err("update")
}

// DeleteWatchlist deletes a watchlist.
func (c *Client) DeleteWatchlist(ctx context.Context, id string) error {
	body := struct {
		ID string `json:"watchlist_id"`
	}{ID: id}
	var out successBody
	if err := c.post(ctx, "/market-data/watchlists/delete", body, &out); err != nil {
		return err
	}
	return out.err("delete")
}

// WatchlistEntry identifies an instrument in a watchlist mutation.
type WatchlistEntry struct {
	Symbol string `json:"symbol"`
	// Category defaults to USStock.
	Category Category `json:"category"`
	// Sort orders the instrument within the watchlist; zero omits it.
	Sort int `json:"sort,omitzero"`
}

// WatchlistInstrument is one instrument in a watchlist.
type WatchlistInstrument struct {
	InstrumentID string `json:"instrument_id"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	ExchangeCode string `json:"exchange_code"`
	Sort         int    `json:"sort"`
	AddedTime    Time   `json:"added_time"`
}

// WatchlistInstruments returns the instruments in a watchlist.
func (c *Client) WatchlistInstruments(ctx context.Context, id string) ([]WatchlistInstrument, error) {
	q := query.New()
	q.Set("watchlist_id", id)
	var out struct {
		Instruments []WatchlistInstrument `json:"instruments"`
	}
	if err := c.get(ctx, "/market-data/watchlists/instruments/list", q, &out); err != nil {
		return nil, err
	}
	return out.Instruments, nil
}

// AddWatchlistInstruments adds instruments to a watchlist.
func (c *Client) AddWatchlistInstruments(ctx context.Context, id string, entries []WatchlistEntry) error {
	return c.mutateInstruments(ctx, "add", id, entries)
}

// RemoveWatchlistInstruments removes instruments from a watchlist.
func (c *Client) RemoveWatchlistInstruments(ctx context.Context, id string, entries []WatchlistEntry) error {
	return c.mutateInstruments(ctx, "remove", id, entries)
}

// UpdateWatchlistInstruments reorders instruments within a watchlist using
// each entry's Sort.
func (c *Client) UpdateWatchlistInstruments(ctx context.Context, id string, entries []WatchlistEntry) error {
	return c.mutateInstruments(ctx, "update", id, entries)
}

func (c *Client) mutateInstruments(ctx context.Context, op, id string, entries []WatchlistEntry) error {
	filled := make([]WatchlistEntry, len(entries))
	for i, e := range entries {
		e.Category = category(e.Category)
		filled[i] = e
	}
	body := struct {
		ID          string           `json:"watchlist_id"`
		Instruments []WatchlistEntry `json:"instruments"`
	}{ID: id, Instruments: filled}
	var out successBody
	if err := c.post(ctx, "/market-data/watchlists/instruments/"+op, body, &out); err != nil {
		return err
	}
	return out.err(op + " instruments")
}
