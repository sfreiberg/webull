package marketdata_test

import (
	"context"
	"errors"
	"testing"

	"github.com/sfreiberg/webull/internal/testutil"
	"github.com/sfreiberg/webull/marketdata"
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
	if len(depth.Asks) != 1 || len(depth.Bids) != 1 {
		t.Errorf("depth = %+v", depth)
	}

	ticks, err := c.Ticks(ctx, marketdata.TicksRequest{Symbol: "AAPL", Count: 10, Sessions: []marketdata.TradingSession{marketdata.Regular}})
	if err != nil {
		t.Fatalf("Ticks: %v", err)
	}
	if len(ticks.Ticks) == 0 || ticks.Ticks[0].Time.IsZero() {
		t.Errorf("ticks = %+v", ticks)
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
