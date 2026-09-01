package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestWatchlistsList(t *testing.T) {
	c, _ := newClient(t, "/market-data/watchlists/list", "watchlists.json", 0)
	got, err := c.Watchlists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	w := got[0]
	if w.ID != "12345678" || w.Name != "My Tech Stocks" || w.Sort != 1 || w.CreateTime.IsZero() || w.UpdateTime.IsZero() {
		t.Errorf("watchlist = %+v", w)
	}
}

func TestCreateWatchlist(t *testing.T) {
	c, f := newClient(t, "/market-data/watchlists/create", "watchlist_create.json", 0)
	id, err := c.CreateWatchlist(context.Background(), "My Tech Stocks", 0)
	if err != nil {
		t.Fatal(err)
	}
	if id != "12345678" {
		t.Errorf("id = %q", id)
	}
	var body map[string]any
	_ = json.Unmarshal(f.gotBody, &body)
	if body["name"] != "My Tech Stocks" {
		t.Errorf("body = %v", body)
	}
	if _, has := body["sort"]; has {
		t.Error("unset sort must be omitted")
	}
}

func TestUpdateAndDeleteWatchlist(t *testing.T) {
	c, f := newClient(t, "/market-data/watchlists/update", "watchlist_success.json", 0)
	if err := c.UpdateWatchlist(context.Background(), "12345678", "Renamed", 2); err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.Unmarshal(f.gotBody, &body)
	if body["watchlist_id"] != "12345678" || body["name"] != "Renamed" || body["sort"] != float64(2) {
		t.Errorf("body = %v", body)
	}

	c2, _ := newClient(t, "/market-data/watchlists/delete", "watchlist_success.json", 0)
	if err := c2.DeleteWatchlist(context.Background(), "12345678"); err != nil {
		t.Fatal(err)
	}
}

func TestWatchlistMutationFailureIsAnError(t *testing.T) {
	// The documented envelope and the bare bool the sandbox returns must
	// both decode, in both outcomes.
	for _, file := range []string{"watchlist_fail.json", "watchlist_fail_bare.json"} {
		c, _ := newClient(t, "", file, 0)
		err := c.DeleteWatchlist(context.Background(), "12345678")
		if !errors.Is(err, ErrWatchlistFailed) {
			t.Errorf("%s: a failure receipt must be ErrWatchlistFailed, got %v", file, err)
		}
	}
	for _, file := range []string{"watchlist_success.json", "watchlist_success_bare.json"} {
		c, _ := newClient(t, "", file, 0)
		if err := c.DeleteWatchlist(context.Background(), "12345678"); err != nil {
			t.Errorf("%s: %v", file, err)
		}
	}
}

func TestWatchlistUnknownReceiptFailsLoudly(t *testing.T) {
	// A 200 receipt in neither known shape must be a decode error, not a
	// silent failure verdict.
	c, _ := newClient(t, "", "watchlist_unknown.json", 0)
	err := c.DeleteWatchlist(context.Background(), "12345678")
	if err == nil || errors.Is(err, ErrWatchlistFailed) {
		t.Errorf("an unrecognised receipt must be a decode error, got %v", err)
	}
}

func TestWatchlistInstrumentsUnwrapTheEnvelope(t *testing.T) {
	c, f := newClient(t, "/market-data/watchlists/instruments/list", "watchlist_instruments.json", 0)
	got, err := c.WatchlistInstruments(context.Background(), "12345678")
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["watchlist_id"][0] != "12345678" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if len(got) != 1 || got[0].Symbol != "AAPL" || got[0].Sort != 1 || got[0].AddedTime.IsZero() {
		t.Errorf("instruments = %+v", got)
	}
}

func TestWatchlistInstrumentMutationsDefaultTheCategory(t *testing.T) {
	for _, op := range []string{"add", "remove", "update"} {
		c, f := newClient(t, "/market-data/watchlists/instruments/"+op, "watchlist_success.json", 0)
		entries := []WatchlistEntry{{Symbol: "AAPL"}, {Symbol: "SPY", Category: USETF, Sort: 2}}
		var err error
		switch op {
		case "add":
			err = c.AddWatchlistInstruments(context.Background(), "12345678", entries)
		case "remove":
			err = c.RemoveWatchlistInstruments(context.Background(), "12345678", entries)
		case "update":
			err = c.UpdateWatchlistInstruments(context.Background(), "12345678", entries)
		}
		if err != nil {
			t.Fatalf("%s: %v", op, err)
		}
		var body struct {
			ID          string           `json:"watchlist_id"`
			Instruments []WatchlistEntry `json:"instruments"`
		}
		_ = json.Unmarshal(f.gotBody, &body)
		if body.ID != "12345678" || len(body.Instruments) != 2 {
			t.Fatalf("%s body = %+v", op, body)
		}
		if body.Instruments[0].Category != USStock || body.Instruments[1].Category != USETF {
			t.Errorf("%s categories = %+v", op, body.Instruments)
		}
	}
}

func TestErrorsPropagateFromEveryWatchlistMethod(t *testing.T) {
	assertCategoryErrors(t, func(c *Client, ctx context.Context) map[string]func() error {
		entries := []WatchlistEntry{{Symbol: "A"}}
		return map[string]func() error{
			"Watchlists":              func() error { _, e := c.Watchlists(ctx); return e },
			"CreateWatchlist":         func() error { _, e := c.CreateWatchlist(ctx, "n", 0); return e },
			"UpdateWatchlist":         func() error { return c.UpdateWatchlist(ctx, "1", "n", 0) },
			"DeleteWatchlist":         func() error { return c.DeleteWatchlist(ctx, "1") },
			"WatchlistInstruments":    func() error { _, e := c.WatchlistInstruments(ctx, "1"); return e },
			"AddWatchlistInstruments": func() error { return c.AddWatchlistInstruments(ctx, "1", entries) },
			"RemoveWatchlistInstrs":   func() error { return c.RemoveWatchlistInstruments(ctx, "1", entries) },
			"UpdateWatchlistInstrs":   func() error { return c.UpdateWatchlistInstruments(ctx, "1", entries) },
		}
	})
}
