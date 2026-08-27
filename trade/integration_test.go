package trade_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/internal/testutil"
	"github.com/sfreiberg/webull/trade"
)

// These run against the live sandbox whenever credentials are present and
// skip with a reported reason otherwise. Everything here is read-only.

func newClient(t *testing.T) (*trade.Client, context.Context) {
	t.Helper()
	return testutil.NewIntegrationClient(t).Trade, testutil.IntegrationContext(t)
}

func firstAccount(ctx context.Context, t *testing.T, c *trade.Client) trade.Account {
	t.Helper()
	accounts, err := c.Accounts(ctx)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	if len(accounts) == 0 {
		t.Skip("integration: the sandbox key has no accounts")
	}
	return accounts[0]
}

func TestIntegrationAccountsBalancePositions(t *testing.T) {
	c, ctx := newClient(t)
	acct := firstAccount(ctx, t, c)
	if acct.AccountID == "" || acct.AccountClass == "" {
		t.Errorf("account is missing identifiers: %+v", acct)
	}

	bal, err := c.Balance(ctx, acct.AccountID)
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if bal.Currency == "" || bal.TotalNetLiquidationValue.IsZero() {
		t.Errorf("balance looks empty: %+v", bal)
	}

	if _, err := c.Positions(ctx, acct.AccountID); err != nil {
		t.Fatalf("Positions: %v", err)
	}
}

func TestIntegrationCashActivities(t *testing.T) {
	c, ctx := newClient(t)
	acct := firstAccount(ctx, t, c)
	acts, err := c.CashActivities(ctx, trade.CashActivitiesRequest{
		AccountID: acct.AccountID,
		StartTime: time.Now().Add(-30 * 24 * time.Hour),
		PageSize:  10,
	})
	if err != nil {
		t.Fatalf("CashActivities: %v", err)
	}
	for _, a := range acts {
		if a.BizTime.IsZero() {
			t.Errorf("activity %s has no biz_time", a.ID)
		}
	}
}

func TestIntegrationInstrumentLookups(t *testing.T) {
	c, ctx := newClient(t)

	t.Run("stocks", func(t *testing.T) {
		page, err := c.StockProfiles(ctx, trade.StockProfilesRequest{Symbols: []string{"AAPL"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Profiles) != 1 || page.Profiles[0].Symbol != "AAPL" {
			t.Errorf("profiles = %+v", page.Profiles)
		}
	})
	t.Run("stocks paginate", func(t *testing.T) {
		first, err := c.StockProfiles(ctx, trade.StockProfilesRequest{SubCategory: trade.ETF})
		if err != nil {
			t.Fatal(err)
		}
		if first.PaginationKey == "" {
			t.Skip("ETF browse fits in one page; nothing to paginate")
		}
		second, err := c.StockProfiles(ctx, trade.StockProfilesRequest{SubCategory: trade.ETF, PaginationKey: first.PaginationKey})
		if err != nil {
			t.Fatal(err)
		}
		if len(second.Profiles) == 0 || second.Profiles[0].Symbol == first.Profiles[0].Symbol {
			t.Error("second page should differ from the first")
		}
	})
	t.Run("crypto", func(t *testing.T) {
		page, err := c.CryptoProfiles(ctx, trade.CryptoProfilesRequest{Symbols: []string{"BTCUSD"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Profiles) != 1 || !page.Profiles[0].PriceStep.Valid {
			t.Errorf("profiles = %+v", page.Profiles)
		}
	})
	t.Run("options", func(t *testing.T) {
		page, err := c.OptionContracts(ctx, trade.OptionContractsRequest{UnderlyingSymbols: []string{"AAPL"}, OptionType: trade.Call})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Contracts) == 0 {
			t.Fatal("no contracts")
		}
		for _, k := range page.Contracts[:min(5, len(page.Contracts))] {
			if k.OptionType != trade.Call || k.StrikePrice.IsZero() {
				t.Errorf("contract = %+v", k)
			}
		}
	})
	t.Run("futures", func(t *testing.T) {
		classes, err := c.FuturesProductClasses(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(classes) == 0 {
			t.Fatal("no product classes")
		}
		products, err := c.FuturesProducts(ctx, classes[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(products) == 0 {
			t.Errorf("no products in class %+v", classes[0])
		}
		contracts, err := c.FuturesContracts(ctx, trade.FuturesContractsRequest{Code: "ES"})
		if err != nil {
			t.Fatal(err)
		}
		if len(contracts) == 0 || contracts[0].MinTick.IsZero() {
			t.Errorf("contracts = %+v", contracts)
		}
	})
	t.Run("event contracts", func(t *testing.T) {
		cats, err := c.EventCategories(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(cats) == 0 {
			t.Fatal("no categories")
		}
		series, err := c.EventSeries(ctx, trade.EventSeriesRequest{Category: trade.EventEconomics})
		if err != nil {
			t.Fatal(err)
		}
		if len(series.Series) == 0 {
			t.Fatal("no economics series")
		}
		sym := series.Series[0].Symbol
		if _, err := c.Events(ctx, trade.EventsRequest{SeriesSymbol: sym}); err != nil {
			t.Fatalf("Events(%s): %v", sym, err)
		}
		if _, err := c.EventMarkets(ctx, trade.EventMarketsRequest{SeriesSymbol: sym}); err != nil {
			t.Fatalf("EventMarkets(%s): %v", sym, err)
		}
	})
}

// marginAccount returns the sandbox's individual margin account, which is
// the one that accepts equity and option orders.
func marginAccount(ctx context.Context, t *testing.T, c *trade.Client) trade.Account {
	t.Helper()
	accounts, err := c.Accounts(ctx)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	for _, a := range accounts {
		if a.AccountClass == trade.AccountClassIndividualMargin {
			return a
		}
	}
	t.Skip("integration: no individual margin account in the sandbox")
	return trade.Account{}
}

// TestIntegrationOrderLifecycle places a real order in the sandbox: a $1.00
// limit buy that cannot fill. It is cancelled by the test, and again by a
// cleanup so that a failure part-way through does not leave it working.
func TestIntegrationOrderLifecycle(t *testing.T) {
	c, ctx := newClient(t)
	acct := marginAccount(ctx, t, c)

	order := &trade.Order{
		Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit,
		Quantity: trade.Price("1"), LimitPrice: trade.Price("1.00"),
	}

	preview, err := c.PreviewOrder(ctx, acct.AccountID, order)
	if err != nil {
		t.Fatalf("PreviewOrder: %v", err)
	}
	if preview.EstimatedCost.IsZero() {
		t.Errorf("preview = %+v", preview)
	}

	placed, err := c.PlaceOrder(ctx, acct.AccountID, order)
	if err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	t.Cleanup(func() {
		// Best effort; the happy path cancels below and this returns an
		// error for an order that is already cancelled.
		_, _ = c.CancelOrder(context.Background(), acct.AccountID, order.ClientOrderID)
	})
	if placed.ClientOrderID != order.ClientOrderID || placed.OrderID == "" {
		t.Fatalf("receipt = %+v", placed)
	}

	waitFor := func(want trade.OrderStatus) *trade.OrderInfo {
		t.Helper()
		var last *trade.OrderInfo
		for i := 0; i < 10; i++ {
			g, err := c.Order(ctx, acct.AccountID, order.ClientOrderID)
			if err != nil {
				t.Fatalf("Order: %v", err)
			}
			if len(g.Orders) == 1 {
				last = &g.Orders[0]
				if last.Status == want {
					return last
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		t.Fatalf("order never reached %s; last = %+v", want, last)
		return nil
	}

	info := waitFor(trade.StatusSubmitted)
	if !info.LimitPrice.Valid || !info.LimitPrice.Decimal.Equal(trade.Price("1").Decimal) {
		t.Errorf("LimitPrice = %v", info.LimitPrice)
	}

	open, err := c.OpenOrders(ctx, trade.OrdersRequest{AccountID: acct.AccountID, PageSize: 10})
	if err != nil {
		t.Fatalf("OpenOrders: %v", err)
	}
	found := false
	for _, g := range open {
		if g.ClientOrderID == order.ClientOrderID {
			found = true
		}
	}
	if !found {
		t.Error("placed order not in the open list")
	}

	if _, err := c.ReplaceOrder(ctx, acct.AccountID, trade.OrderModification{
		ClientOrderID: order.ClientOrderID, LimitPrice: trade.Price("1.05"), Quantity: trade.Price("1"),
	}); err != nil {
		t.Fatalf("ReplaceOrder: %v", err)
	}
	time.Sleep(time.Second)
	if g, err := c.Order(ctx, acct.AccountID, order.ClientOrderID); err == nil && len(g.Orders) == 1 {
		if !g.Orders[0].LimitPrice.Decimal.Equal(trade.Price("1.05").Decimal) {
			t.Errorf("replace did not take: LimitPrice = %v", g.Orders[0].LimitPrice)
		}
	}

	if _, err := c.CancelOrder(ctx, acct.AccountID, order.ClientOrderID); err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	waitFor(trade.StatusCancelled)

	if _, err := c.OrderHistory(ctx, trade.OrdersRequest{AccountID: acct.AccountID, PageSize: 10}); err != nil {
		t.Fatalf("OrderHistory: %v", err)
	}
}

func TestIntegrationOptionPreview(t *testing.T) {
	c, ctx := newClient(t)
	acct := marginAccount(ctx, t, c)

	chain, err := c.OptionContracts(ctx, trade.OptionContractsRequest{UnderlyingSymbols: []string{"AAPL"}, OptionType: trade.Call})
	if err != nil {
		t.Fatalf("OptionContracts: %v", err)
	}
	if len(chain.Contracts) == 0 {
		t.Skip("integration: no AAPL calls listed")
	}
	leg, err := trade.LegFromSymbol(chain.Contracts[0].Symbol)
	if err != nil {
		t.Fatalf("LegFromSymbol(%s): %v", chain.Contracts[0].Symbol, err)
	}

	preview, err := c.PreviewOrder(ctx, acct.AccountID, &trade.Order{
		Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit,
		Quantity: trade.Price("1"), LimitPrice: trade.Price("0.05"),
		PositionIntent: trade.BuyToOpen, Legs: []trade.OrderLeg{leg},
	})
	if err != nil {
		t.Fatalf("PreviewOrder: %v", err)
	}
	// One contract at $0.05 with a 100 multiplier.
	if !preview.EstimatedCost.Equal(trade.Price("5").Decimal) {
		t.Errorf("EstimatedCost = %v, want 5", preview.EstimatedCost)
	}
}

func TestIntegrationServerRejectionIsClassified(t *testing.T) {
	c, ctx := newClient(t)
	acct := marginAccount(ctx, t, c)

	// Bypass local validation to see how the server reports a bad leg.
	_, err := c.PreviewOrder(ctx, acct.AccountID, &trade.Order{
		Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit, InstrumentType: trade.InstrumentOption,
		Quantity: trade.Price("1"), LimitPrice: trade.Price("1"),
		Legs:          []trade.OrderLeg{{Symbol: "AAPL", OptionType: trade.Call, ExpireDate: "2026-12-18", StrikePrice: trade.Price("240")}},
		ClientOrderID: "integration-badleg-0001", TradingSession: trade.SessionCore,
		OptionStrategy: "NOT_A_STRATEGY",
	})
	if err == nil {
		t.Skip("integration: the server accepted an order this test expected it to reject")
	}
	if !errors.Is(err, webull.ErrInvalidRequest) {
		t.Errorf("server rejection did not classify as ErrInvalidRequest: %v", err)
	}
}
