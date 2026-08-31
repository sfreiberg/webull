package marketdata

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestOptionSnapshotsDecodeGreeks(t *testing.T) {
	c, f := newClient(t, "/market-data/options/snapshots/list", "option_snapshots.json", 0)
	got, err := c.OptionSnapshots(context.Background(), []string{"AAPL261218C00240000", "AAPL261218P00240000"})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["category"][0] != "US_OPTION" || f.gotQuery["symbols"][0] != "AAPL261218C00240000,AAPL261218P00240000" {
		t.Errorf("query = %v", f.gotQuery)
	}
	o := got[0]
	if !o.Delta.Valid || !o.Delta.Decimal.Equal(d("0.9571")) || !o.StrikePrice.Decimal.Equal(d("240")) || !o.DealAmount.Valid || o.LastTradeTime.IsZero() {
		t.Errorf("snapshot = %+v", o)
	}
}

func TestOptionTicksCamelCaseInstrumentID(t *testing.T) {
	c, f := newClient(t, "/market-data/options/ticks/list", "option_ticks.json", 0)
	got, err := c.OptionTicks(context.Background(), "AAPL261218C00240000", 10)
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["count"][0] != "10" || f.gotQuery["category"][0] != "US_OPTION" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if got.InstrumentID != "1044605620" {
		t.Errorf("instrumentId (camel case on the wire) did not decode: %+v", got)
	}
	if len(got.Ticks) == 0 || got.Ticks[0].Side != "NS" || got.Ticks[0].Time.IsZero() {
		t.Errorf("ticks = %+v", got.Ticks)
	}
}

func TestAssetBarsAreABareArray(t *testing.T) {
	paths := map[string]string{
		"option_bars": "/market-data/options/bars/list", "futures_bars": "/market-data/futures/bars/list",
		"crypto_bars": "/market-data/crypto/bars/list", "event_bars": "/market-data/event-contracts/bars/list",
	}
	cats := map[string]string{"option_bars": "US_OPTION", "futures_bars": "US_FUTURES", "crypto_bars": "US_CRYPTO", "event_bars": "US_EVENT"}
	for name, call := range map[string]func(*Client) ([]Bars, error){
		"option_bars": func(c *Client) ([]Bars, error) {
			return c.OptionBars(context.Background(), AssetBarsRequest{Symbols: []string{"X"}, Timespan: Daily, Count: 3})
		},
		"futures_bars": func(c *Client) ([]Bars, error) {
			return c.FuturesBars(context.Background(), AssetBarsRequest{Symbols: []string{"X"}, Timespan: Daily})
		},
		"crypto_bars": func(c *Client) ([]Bars, error) {
			return c.CryptoBars(context.Background(), AssetBarsRequest{Symbols: []string{"X"}, Timespan: Daily})
		},
		"event_bars": func(c *Client) ([]Bars, error) {
			return c.EventBars(context.Background(), AssetBarsRequest{Symbols: []string{"X"}, Timespan: Daily})
		},
	} {
		t.Run(name, func(t *testing.T) {
			c, f := newClient(t, paths[name], name+".json", 0)
			got, err := call(c)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || len(got[0].Bars) == 0 || got[0].Bars[0].Time.IsZero() || got[0].Bars[0].Open.IsZero() {
				t.Errorf("bars = %+v", got)
			}
			if f.gotQuery["timespan"][0] != "D" || f.gotQuery["category"][0] != cats[name] {
				t.Errorf("query = %v", f.gotQuery)
			}
		})
	}
}

func TestBarsAcceptTheDocumentedEnvelopeToo(t *testing.T) {
	// The sandbox returns a bare array; the documentation shows an envelope.
	// Both must decode, since production has never been observed.
	c, _ := newClient(t, "", "bars.json", 0) // the stock fixture is the envelope form
	got, err := c.OptionBars(context.Background(), AssetBarsRequest{Symbols: []string{"X"}, Timespan: Daily})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Symbol != "AAPL" {
		t.Errorf("enveloped bars = %+v", got)
	}
}

func TestFuturesBarsNeverSendRealTimeFlag(t *testing.T) {
	c, f := newClient(t, "", "futures_bars.json", 0)
	if _, err := c.FuturesBars(context.Background(), AssetBarsRequest{Symbols: []string{"MESmain"}, Timespan: Daily, Completed: true}); err != nil {
		t.Fatal(err)
	}
	if _, has := f.gotQuery["real_time_required"]; has {
		t.Error("futures bars do not document real_time_required and must not send it")
	}
}

func TestEventBarsSendRealTimeFlagBothWays(t *testing.T) {
	c, f := newClient(t, "", "event_bars.json", 0)
	if _, err := c.EventBars(context.Background(), AssetBarsRequest{Symbols: []string{"X"}, Timespan: Daily, Completed: true}); err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["real_time_required"][0] != "false" {
		t.Errorf("query = %v", f.gotQuery)
	}
}

func TestAssetBarsRealTimeFlag(t *testing.T) {
	c, f := newClient(t, "", "crypto_bars.json", 0)
	// Crypto requires the flag; unset Completed means include the live bar.
	if _, err := c.CryptoBars(context.Background(), AssetBarsRequest{Symbols: []string{"BTCUSD"}, Timespan: Daily}); err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["real_time_required"][0] != "true" {
		t.Errorf("crypto must always send the flag: %v", f.gotQuery)
	}
	c2, f2 := newClient(t, "", "option_bars.json", 0)
	if _, err := c2.OptionBars(context.Background(), AssetBarsRequest{Symbols: []string{"X"}, Timespan: Daily}); err != nil {
		t.Fatal(err)
	}
	if _, has := f2.gotQuery["real_time_required"]; has {
		t.Error("options omit the flag unless Completed is set")
	}
	if _, err := c2.OptionBars(context.Background(), AssetBarsRequest{Symbols: []string{"X"}, Timespan: Daily, Completed: true}); err != nil {
		t.Fatal(err)
	}
	if f2.gotQuery["real_time_required"][0] != "false" {
		t.Errorf("Completed should send false: %v", f2.gotQuery)
	}
}

func TestFuturesSnapshotsAndTicks(t *testing.T) {
	c, f := newClient(t, "/market-data/futures/snapshots/list", "futures_snapshots.json", 0)
	snaps, err := c.FuturesSnapshots(context.Background(), []string{"MESmain"})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["category"][0] != "US_FUTURES" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if snaps[0].SettleDate.IsZero() {
		t.Error("settle_date did not decode")
	}
	// A continuous symbol reports the resolved contract.
	if snaps[0].Symbol != "MESU6" || !snaps[0].Price.Decimal.Equal(d("7732")) || !snaps[0].OpenInterest.Valid {
		t.Errorf("snapshot = %+v", snaps[0])
	}
	c2, f2 := newClient(t, "/market-data/futures/ticks/list", "futures_ticks.json", 0)
	ticks, err := c2.FuturesTicks(context.Background(), "MESmain", 5)
	if err != nil {
		t.Fatal(err)
	}
	if f2.gotQuery["category"][0] != "US_FUTURES" || ticks.InstrumentID != "470005101" || len(ticks.Ticks) == 0 || ticks.Ticks[0].Side != TickBuy {
		t.Errorf("ticks = %+v", ticks)
	}
}

func TestFuturesDepthRequiresLevel2(t *testing.T) {
	c, f := newClient(t, "/market-data/futures/depths/list", "error_futures_lv2.json", http.StatusForbidden)
	_, err := c.FuturesDepth(context.Background(), "MESmain", 10)
	if !errors.Is(err, ErrNotSubscribed) {
		t.Fatalf("got %v", err)
	}
	if f.gotQuery["depth"][0] != "10" || f.gotQuery["category"][0] != "US_FUTURES" {
		t.Errorf("query = %v", f.gotQuery)
	}
}

func TestFuturesFootprintsForceCategory(t *testing.T) {
	c, f := newClient(t, "/market-data/futures/footprints/list", "footprints.json", 0)
	if _, err := c.FuturesFootprints(context.Background(), FootprintsRequest{Symbols: []string{"MESmain"}, Timespan: Minute5, Category: USStock}); err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["category"][0] != "US_FUTURES" {
		t.Errorf("category = %v; the futures method must not send a stock category", f.gotQuery["category"])
	}
}

func TestCryptoSnapshotsAndBars(t *testing.T) {
	c, f := newClient(t, "/market-data/crypto/snapshots/list", "crypto_snapshots.json", 0)
	snaps, err := c.CryptoSnapshots(context.Background(), []string{"BTCUSD", "ETHUSD"})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["category"][0] != "US_CRYPTO" || snaps[0].Symbol != "BTCUSD" || !snaps[0].BidSize.Decimal.Equal(d("0.000022")) {
		t.Errorf("snapshot = %+v", snaps[0])
	}
	c2, _ := newClient(t, "", "crypto_bars.json", 0)
	bars, err := c2.CryptoBars(context.Background(), AssetBarsRequest{Symbols: []string{"BTCUSD"}, Timespan: Daily})
	if err != nil {
		t.Fatal(err)
	}
	if !bars[0].Bars[0].Volume.IsZero() {
		t.Error("crypto bars carry no volume; a nonzero value means the fixture changed")
	}
}

func TestEventSnapshotDepthTicks(t *testing.T) {
	c, f0 := newClient(t, "/market-data/event-contracts/snapshots/list", "event_snapshots.json", 0)
	snaps, err := c.EventSnapshots(context.Background(), []string{"KXRATECUTCOUNT-26DEC31-T6"})
	if err != nil {
		t.Fatal(err)
	}
	if f0.gotQuery["category"][0] != "US_EVENT" {
		t.Errorf("query = %v", f0.gotQuery)
	}
	s := snaps[0]
	// last_trade_time is a string on the wire here, an integer elsewhere.
	if s.LastTradeTime.IsZero() || !s.YesAsk.Valid || s.YesBid.Valid || s.Name == "" {
		t.Errorf("snapshot = %+v", s)
	}

	c2, f := newClient(t, "/market-data/event-contracts/depths/list", "event_depth.json", 0)
	depth, err := c2.EventDepth(context.Background(), "KXRATECUTCOUNT-26DEC31-T6", 10)
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["depth"][0] != "10" || len(depth.NoBids) == 0 || len(depth.YesBids) != 0 || depth.QuoteTime.IsZero() {
		t.Errorf("depth = %+v", depth)
	}

	c3, _ := newClient(t, "/market-data/event-contracts/ticks/list", "event_ticks.json", 0)
	ticks, err := c3.EventTicks(context.Background(), "KXRATECUTCOUNT-26DEC31-T6", 10)
	if err != nil {
		t.Fatal(err)
	}
	tk := ticks.Ticks[0]
	if tk.Side != OutcomeNo || tk.TradeID == "" || !tk.YesPrice.Equal(d("0.001")) || !tk.NoPrice.Equal(d("0.999")) {
		t.Errorf("tick = %+v", tk)
	}
}

func TestAssetErrorsPropagate(t *testing.T) {
	assertCategoryErrors(t, func(c *Client, ctx context.Context) map[string]func() error {
		req := AssetBarsRequest{Symbols: []string{"X"}, Timespan: Daily}
		return map[string]func() error{
			"OptionSnapshots":  func() error { _, e := c.OptionSnapshots(ctx, []string{"X"}); return e },
			"OptionTicks":      func() error { _, e := c.OptionTicks(ctx, "X", 1); return e },
			"OptionBars":       func() error { _, e := c.OptionBars(ctx, req); return e },
			"FuturesSnapshots": func() error { _, e := c.FuturesSnapshots(ctx, []string{"X"}); return e },
			"FuturesTicks":     func() error { _, e := c.FuturesTicks(ctx, "X", 1); return e },
			"FuturesDepth":     func() error { _, e := c.FuturesDepth(ctx, "X", 1); return e },
			"FuturesBars":      func() error { _, e := c.FuturesBars(ctx, req); return e },
			"FuturesFootprints": func() error {
				_, e := c.FuturesFootprints(ctx, FootprintsRequest{Symbols: []string{"X"}, Timespan: Minute})
				return e
			},
			"CryptoSnapshots": func() error { _, e := c.CryptoSnapshots(ctx, []string{"X"}); return e },
			"CryptoBars":      func() error { _, e := c.CryptoBars(ctx, req); return e },
			"EventSnapshots":  func() error { _, e := c.EventSnapshots(ctx, []string{"X"}); return e },
			"EventDepth":      func() error { _, e := c.EventDepth(ctx, "X", 1); return e },
			"EventTicks":      func() error { _, e := c.EventTicks(ctx, "X", 1); return e },
			"EventBars":       func() error { _, e := c.EventBars(ctx, req); return e },
		}
	})
}
