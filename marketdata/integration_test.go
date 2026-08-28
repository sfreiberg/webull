package marketdata_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sfreiberg/webull/internal/testutil"
	"github.com/sfreiberg/webull/marketdata"
	"github.com/sfreiberg/webull/trade"
)

func newClient(t *testing.T) (*marketdata.Client, context.Context) {
	t.Helper()
	return testutil.NewIntegrationClient(t).MarketData, testutil.IntegrationContext(t)
}

func TestIntegrationSnapshotsDepthTicksBars(t *testing.T) {
	c, ctx := newClient(t)

	snaps, err := c.Snapshots(ctx, marketdata.SnapshotsRequest{Symbols: []string{"AAPL", "SPY"}, ExtendedHours: true, Overnight: true})
	if err != nil {
		t.Fatalf("Snapshots: %v", err)
	}
	if len(snaps) != 2 || !snaps[0].Price.Valid || snaps[0].LastTradeTime.IsZero() {
		t.Errorf("snapshots = %+v", snaps)
	}

	// Level 1 is what a sandbox key is entitled to; deeper is rejected as a
	// parameter error, not an entitlement error.
	depth, err := c.Depth(ctx, marketdata.DepthRequest{Symbol: "AAPL", Levels: 1})
	if err != nil {
		t.Fatalf("Depth: %v", err)
	}
	if len(depth.Asks) == 0 && len(depth.Bids) == 0 {
		t.Skip("integration: sandbox returned an empty book for AAPL")
	}

	ticks, err := c.Ticks(ctx, marketdata.TicksRequest{Symbol: "AAPL", Count: 10, Sessions: []marketdata.TradingSession{marketdata.Regular}})
	if err != nil {
		t.Fatalf("Ticks: %v", err)
	}
	if len(ticks.Ticks) == 0 {
		t.Skip("integration: sandbox returned no ticks for AAPL")
	}
	if ticks.Ticks[0].Time.IsZero() {
		t.Errorf("tick time did not decode: %+v", ticks.Ticks[0])
	}

	bars, err := c.Bars(ctx, marketdata.BarsRequest{Symbols: []string{"AAPL", "MSFT"}, Timespan: marketdata.Daily, Count: 3})
	if err != nil {
		t.Fatalf("Bars: %v", err)
	}
	if len(bars) != 2 || len(bars[0].Bars) == 0 || bars[0].Bars[0].Time.IsZero() {
		t.Errorf("bars = %+v", bars)
	}
}

func TestIntegrationFundamentals(t *testing.T) {
	c, ctx := newClient(t)
	if cp, err := c.CompanyProfile(ctx, "AAPL", ""); err != nil || cp.CompanyName == "" {
		t.Errorf("CompanyProfile: %v %+v", err, cp)
	}
	if r, err := c.AnalystRating(ctx, "AAPL", ""); err != nil || r.Analysts.IsZero() {
		t.Errorf("AnalystRating: %v %+v", err, r)
	}
	if tp, err := c.TargetPrice(ctx, "AAPL", ""); err != nil || tp.Mean.IsZero() {
		t.Errorf("TargetPrice: %v %+v", err, tp)
	}
}

// TestIntegrationEntitlements records what the key is not subscribed to.
// Each unsubscribed product skips with Webull's own message; if a product
// becomes available, the test passes and the matrix should be updated.
func TestIntegrationEntitlements(t *testing.T) {
	c, ctx := newClient(t)
	for name, call := range map[string]func() error{
		"footprints": func() error {
			_, err := c.Footprints(ctx, marketdata.FootprintsRequest{Symbols: []string{"AAPL"}, Timespan: marketdata.Minute5, Count: 3, Session: marketdata.Regular})
			return err
		},
		"imbalance": func() error {
			_, err := c.ImbalanceSnapshot(ctx, marketdata.ImbalanceRequest{Symbol: "AAPL", Type: marketdata.PreClose})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			if errors.Is(err, marketdata.ErrNotSubscribed) {
				t.Skipf("integration: %v", err)
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			t.Logf("%s is available to this key; update the compatibility matrix", name)
		})
	}
}

// liveOptionSymbol returns a currently listed AAPL call, so the test does
// not expire with a hard-coded contract.
func liveOptionSymbol(t *testing.T) string {
	t.Helper()
	tc := testutil.NewIntegrationClient(t).Trade
	chain, err := tc.OptionContracts(testutil.IntegrationContext(t), trade.OptionContractsRequest{UnderlyingSymbols: []string{"AAPL"}, OptionType: trade.Call})
	if err != nil || len(chain.Contracts) == 0 {
		t.Skipf("integration: no AAPL calls listed (%v)", err)
	}
	return chain.Contracts[0].Symbol
}

// liveEventSymbol returns a currently tradable event market.
func liveEventSymbol(t *testing.T) string {
	t.Helper()
	tc := testutil.NewIntegrationClient(t).Trade
	markets, err := tc.EventMarkets(testutil.IntegrationContext(t), trade.EventMarketsRequest{SeriesSymbol: "KXRATECUTCOUNT"})
	if err != nil {
		t.Skipf("integration: EventMarkets: %v", err)
	}
	for _, m := range markets.Markets {
		if m.TradableStatus == trade.Tradable {
			return m.Symbol
		}
	}
	t.Skip("integration: no tradable event market in the series")
	return ""
}

func TestIntegrationOptionMarketData(t *testing.T) {
	c, ctx := newClient(t)
	sym := liveOptionSymbol(t)
	snaps, err := c.OptionSnapshots(ctx, []string{sym})
	if err != nil {
		t.Fatalf("OptionSnapshots: %v", err)
	}
	if len(snaps) != 1 || !snaps[0].StrikePrice.Valid || !snaps[0].Delta.Valid {
		t.Errorf("snapshots = %+v", snaps)
	}
	ticks, err := c.OptionTicks(ctx, sym, 5)
	if err != nil {
		t.Fatalf("OptionTicks: %v", err)
	}
	if ticks.InstrumentID == "" {
		t.Errorf("instrumentId did not decode: %+v", ticks)
	}
	bars, err := c.OptionBars(ctx, marketdata.AssetBarsRequest{Symbols: []string{sym}, Timespan: marketdata.Daily, Count: 3})
	if err != nil {
		t.Fatalf("OptionBars: %v", err)
	}
	if len(bars) != 1 || len(bars[0].Bars) == 0 {
		t.Errorf("bars = %+v", bars)
	}
}

func TestIntegrationFuturesMarketData(t *testing.T) {
	c, ctx := newClient(t)
	// The sandbox serves exactly one futures symbol.
	snaps, err := c.FuturesSnapshots(ctx, []string{"MESmain"})
	if err != nil {
		t.Fatalf("FuturesSnapshots: %v", err)
	}
	if len(snaps) != 1 || !snaps[0].Price.Valid {
		t.Errorf("snapshots = %+v", snaps)
	}
	if _, err := c.FuturesTicks(ctx, "MESmain", 5); err != nil {
		t.Fatalf("FuturesTicks: %v", err)
	}
	if bars, err := c.FuturesBars(ctx, marketdata.AssetBarsRequest{Symbols: []string{"MESmain"}, Timespan: marketdata.Daily, Count: 3}); err != nil || len(bars) != 1 {
		t.Fatalf("FuturesBars: %v %+v", err, bars)
	}
	for name, call := range map[string]func() error{
		"depth": func() error { _, e := c.FuturesDepth(ctx, "MESmain", 1); return e },
		"footprints": func() error {
			_, e := c.FuturesFootprints(ctx, marketdata.FootprintsRequest{Symbols: []string{"MESmain"}, Timespan: marketdata.Minute5, Count: 3, Session: marketdata.Regular})
			return e
		},
	} {
		t.Run(name, func(t *testing.T) {
			err := call()
			if errors.Is(err, marketdata.ErrNotSubscribed) {
				t.Skipf("integration: %v", err)
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestIntegrationCryptoMarketData(t *testing.T) {
	c, ctx := newClient(t)
	snaps, err := c.CryptoSnapshots(ctx, []string{"BTCUSD", "ETHUSD"})
	if err != nil {
		t.Fatalf("CryptoSnapshots: %v", err)
	}
	if len(snaps) != 2 || !snaps[0].Price.Valid {
		t.Errorf("snapshots = %+v", snaps)
	}
	bars, err := c.CryptoBars(ctx, marketdata.AssetBarsRequest{Symbols: []string{"BTCUSD"}, Timespan: marketdata.Daily, Count: 3})
	if err != nil {
		t.Fatalf("CryptoBars: %v", err)
	}
	if len(bars) != 1 || len(bars[0].Bars) == 0 {
		t.Errorf("bars = %+v", bars)
	}
}

func TestIntegrationEventMarketData(t *testing.T) {
	c, ctx := newClient(t)
	sym := liveEventSymbol(t)
	snaps, err := c.EventSnapshots(ctx, []string{sym})
	if err != nil {
		t.Fatalf("EventSnapshots: %v", err)
	}
	if len(snaps) != 1 || snaps[0].Name == "" {
		t.Errorf("snapshots = %+v", snaps)
	}
	if _, err := c.EventDepth(ctx, sym, 5); err != nil {
		t.Fatalf("EventDepth: %v", err)
	}
	ticks, err := c.EventTicks(ctx, sym, 5)
	if err != nil {
		t.Fatalf("EventTicks: %v", err)
	}
	if len(ticks.Ticks) > 0 && ticks.Ticks[0].TradeID == "" {
		t.Errorf("tick = %+v", ticks.Ticks[0])
	}
	if bars, err := c.EventBars(ctx, marketdata.AssetBarsRequest{Symbols: []string{sym}, Timespan: marketdata.Daily, Count: 3}); err != nil || len(bars) != 1 {
		t.Fatalf("EventBars: %v %+v", err, bars)
	}
}
