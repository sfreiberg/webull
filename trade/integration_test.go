package trade_test

import (
	"context"
	"errors"
	"strings"
	"sync"
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

// accountList is fetched once per test binary. Every integration test needs
// it, and fetching it per test trips the sandbox's rate limit on the
// accounts endpoint when the suite is run repeatedly.
var (
	accountsOnce sync.Once
	accountList  []trade.Account
	accountsErr  error
)

func cachedAccounts(ctx context.Context, c *trade.Client) ([]trade.Account, error) {
	accountsOnce.Do(func() { accountList, accountsErr = c.Accounts(ctx) })
	return accountList, accountsErr
}

// accountWhere returns the first account matching pred, or skips.
func accountWhere(ctx context.Context, t *testing.T, c *trade.Client, what string, pred func(trade.Account) bool) trade.Account {
	t.Helper()
	accounts, err := cachedAccounts(ctx, c)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	for _, a := range accounts {
		if pred(a) {
			return a
		}
	}
	t.Skipf("integration: the sandbox key has no %s", what)
	return trade.Account{}
}

func firstAccount(ctx context.Context, t *testing.T, c *trade.Client) trade.Account {
	t.Helper()
	return accountWhere(ctx, t, c, "accounts", func(trade.Account) bool { return true })
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
	return accountWhere(ctx, t, c, "individual margin account",
		func(a trade.Account) bool { return a.AccountClass == trade.AccountClassIndividualMargin })
}

// TestIntegrationOrderLifecycle places a real order in the sandbox: a $1.00
// limit buy that cannot fill. It is cancelled by the test, and again by a
// cleanup so that a failure part-way through does not leave it working.
//
// The order is GTC: the sandbox enforces regular trading hours for DAY
// orders, and these tests must run at any time of day.
func TestIntegrationOrderLifecycle(t *testing.T) {
	c, ctx := newClient(t)
	acct := marginAccount(ctx, t, c)

	order := &trade.Order{
		Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit, TimeInForce: trade.GTC,
		Quantity: trade.Price("1"), LimitPrice: trade.Price("1.00"),
	}

	var preview *trade.OrderPreview
	if err := retryTransient(ctx, t, "PreviewOrder", func() error {
		var e error
		preview, e = c.PreviewOrder(ctx, acct.AccountID, order)
		return e
	}); err != nil {
		skipIfMarketClosed(t, err)
		t.Fatalf("PreviewOrder: %v", err)
	}
	if preview.EstimatedCost.IsZero() {
		t.Errorf("preview = %+v", preview)
	}

	var placed *trade.OrderReceipt
	if err := retryTransient(ctx, t, "PlaceOrder", func() error {
		var e error
		placed, e = c.PlaceOrder(ctx, acct.AccountID, order)
		return e
	}); err != nil {
		skipIfMarketClosed(t, err)
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

	// waitFor polls until the order satisfies pred, honouring the context.
	waitFor := func(what string, pred func(trade.OrderInfo) bool) *trade.OrderInfo {
		t.Helper()
		var last *trade.OrderInfo
		for {
			g, err := readOrder(ctx, c, acct.AccountID, order.ClientOrderID)
			if err != nil {
				t.Fatalf("order never %s: %v; last = %+v", what, err, last)
			}
			if len(g.Orders) == 1 {
				last = &g.Orders[0]
				if pred(*last) {
					return last
				}
			}
			select {
			case <-ctx.Done():
				t.Fatalf("order never %s; last = %+v", what, last)
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
	status := func(want trade.OrderStatus) func(trade.OrderInfo) bool {
		return func(o trade.OrderInfo) bool { return o.Status == want }
	}

	info := waitFor("submitted", status(trade.StatusSubmitted))
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

	// A price-only replace: quantity is deliberately omitted, which the
	// sandbox accepts.
	if _, err := c.ReplaceOrder(ctx, acct.AccountID, trade.OrderModification{
		ClientOrderID: order.ClientOrderID, LimitPrice: trade.Price("1.05"),
	}); err != nil {
		t.Fatalf("ReplaceOrder: %v", err)
	}
	waitFor("repriced to 1.05", func(o trade.OrderInfo) bool {
		return o.LimitPrice.Valid && o.LimitPrice.Decimal.Equal(trade.Price("1.05").Decimal)
	})

	if _, err := c.CancelOrder(ctx, acct.AccountID, order.ClientOrderID); err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	waitFor("cancelled", status(trade.StatusCancelled))

	// The ID is now consumed: placing the same Order again is a duplicate.
	if _, err := c.PlaceOrder(ctx, acct.AccountID, order); !errors.Is(err, trade.ErrDuplicateOrder) {
		t.Errorf("re-placing a consumed order: got %v, want ErrDuplicateOrder", err)
	}

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

	preview, err := previewOrder(ctx, t, c, acct.AccountID, &trade.Order{
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

	// A leg that passes local validation but names a contract that does not
	// exist, so the rejection comes from the server.
	_, err := previewOrder(ctx, t, c, acct.AccountID, &trade.Order{
		Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit,
		Quantity: trade.Price("1"), LimitPrice: trade.Price("1"),
		Legs: []trade.OrderLeg{{Symbol: "AAPL", OptionType: trade.Call, ExpireDate: "2026-12-18", StrikePrice: trade.Price("999999")}},
	})
	if err == nil {
		t.Fatal("the server accepted a contract at strike 999999; pick a different deterministic rejection")
	}
	if !errors.Is(err, webull.ErrInvalidRequest) {
		t.Errorf("server rejection did not classify as ErrInvalidRequest: %v", err)
	}
}

// TestIntegrationBracketLifecycle places a real bracket in the sandbox: a
// $1.00 limit buy that cannot fill, with a take-profit and a stop-loss
// attached. Cancelling the master cancels the group.
func TestIntegrationBracketLifecycle(t *testing.T) {
	c, ctx := newClient(t)
	acct := marginAccount(ctx, t, c)

	// GTC throughout: the sandbox rejects DAY orders outside regular hours.
	combo := trade.Bracket(
		&trade.Order{Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit, TimeInForce: trade.GTC, Quantity: trade.Price("1"), LimitPrice: trade.Price("1.00")},
		&trade.Order{Symbol: "AAPL", Side: trade.Sell, Type: trade.Limit, TimeInForce: trade.GTC, Quantity: trade.Price("1"), LimitPrice: trade.Price("999")},
		&trade.Order{Symbol: "AAPL", Side: trade.Sell, Type: trade.StopLoss, TimeInForce: trade.GTC, Quantity: trade.Price("1"), StopPrice: trade.Price("0.50")},
	)
	if err := retryTransient(ctx, t, "PreviewCombo", func() error {
		_, e := c.PreviewCombo(ctx, acct.AccountID, combo)
		return e
	}); err != nil {
		skipIfMarketClosed(t, err)
		t.Fatalf("PreviewCombo: %v", err)
	}

	var receipt *trade.OrderReceipt
	err := retryTransient(ctx, t, "PlaceCombo", func() error {
		var e error
		receipt, e = c.PlaceCombo(ctx, acct.AccountID, combo)
		return e
	})
	if err != nil {
		skipIfMarketClosed(t, err)
		t.Fatalf("PlaceCombo: %v", err)
	}
	t.Cleanup(func() { _ = c.CancelCombo(context.Background(), acct.AccountID, combo) })
	if receipt.ComboOrderID == "" || receipt.ClientComboOrderID != combo.ClientComboOrderID {
		t.Fatalf("receipt = %+v", receipt)
	}

	// Each sub-order is retrievable by its own ID, carrying its role.
	time.Sleep(time.Second)
	for _, o := range combo.Orders {
		g, err := c.Order(ctx, acct.AccountID, o.ClientOrderID)
		if err != nil {
			t.Fatalf("Order(%s): %v", o.ComboType, err)
		}
		if g.ComboType != o.ComboType || g.ComboOrderID != receipt.ComboOrderID {
			t.Errorf("%s: group = %+v", o.ComboType, g)
		}
	}

	if err := retryTransient(ctx, t, "CancelCombo", func() error {
		return c.CancelCombo(ctx, acct.AccountID, combo)
	}); err != nil {
		t.Fatalf("CancelCombo: %v", err)
	}
	time.Sleep(time.Second)
	g, err := c.Order(ctx, acct.AccountID, combo.Orders[0].ClientOrderID)
	if err != nil {
		t.Fatalf("Order after cancel: %v", err)
	}
	if len(g.Orders) != 1 || g.Orders[0].Status != trade.StatusCancelled {
		t.Errorf("master after cancel = %+v", g)
	}
}

func TestIntegrationMultiLegPreviews(t *testing.T) {
	c, ctx := newClient(t)
	acct := marginAccount(ctx, t, c)
	call := func(side trade.Side, strike string, kind trade.OptionType) trade.OrderLeg {
		return trade.OrderLeg{Symbol: "AAPL", Side: side, OptionType: kind, ExpireDate: "2026-12-18", StrikePrice: trade.Price(strike)}
	}
	for name, o := range map[string]*trade.Order{
		"vertical": {Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit, Quantity: trade.Price("1"), LimitPrice: trade.Price("0.50"),
			OptionStrategy: trade.StrategyVertical, Legs: []trade.OrderLeg{call(trade.Buy, "240", trade.Call), call(trade.Sell, "250", trade.Call)}},
		"iron condor": {Symbol: "AAPL", Side: trade.Sell, Type: trade.Limit, Quantity: trade.Price("1"), LimitPrice: trade.Price("0.50"),
			OptionStrategy: trade.StrategyIronCondor, Legs: []trade.OrderLeg{
				call(trade.Buy, "180", trade.Put), call(trade.Sell, "200", trade.Put), call(trade.Sell, "260", trade.Call), call(trade.Buy, "280", trade.Call)}},
		"covered stock": {Symbol: "AAPL", Side: trade.Buy, Type: trade.Market, Quantity: trade.Price("1"),
			OptionStrategy: trade.StrategyCoveredStock, Legs: []trade.OrderLeg{
				{Symbol: "AAPL", Side: trade.Buy, Quantity: trade.Price("100"), InstrumentType: trade.InstrumentEquity}, call(trade.Sell, "260", trade.Call)}},
		"straddle": {Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit, Quantity: trade.Price("1"), LimitPrice: trade.Price("1.00"),
			OptionStrategy: trade.StrategyStraddle, Legs: []trade.OrderLeg{call(trade.Buy, "240", trade.Call), call(trade.Buy, "240", trade.Put)}},
		"trailing stop": {Symbol: "AAPL", Side: trade.Buy, Type: trade.TrailingStopLoss, Quantity: trade.Price("1"),
			TrailingType: trade.TrailByPercentage, TrailingStopStep: trade.Price("0.05")},
	} {
		t.Run(name, func(t *testing.T) {
			p, err := previewOrder(ctx, t, c, acct.AccountID, o)
			if err != nil {
				t.Fatalf("PreviewOrder: %v", err)
			}
			if p.EstimatedCost.IsZero() {
				t.Errorf("preview = %+v", p)
			}
		})
	}
}

// TestIntegrationOTOGroupsInSandbox records the sandbox's behaviour for
// OTO, OCO and OTOCO: at the time of writing it rejects all three with
// "invalid combo_type", contrary to the documentation. The test skips with
// that reason so the limitation is visible in every run, and fails if the
// rejection ever takes a different shape.
func TestIntegrationOTOGroupsInSandbox(t *testing.T) {
	c, ctx := newClient(t)
	acct := marginAccount(ctx, t, c)
	buy := func(p string) *trade.Order {
		return &trade.Order{Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit, Quantity: trade.Price("1"), LimitPrice: trade.Price(p)}
	}
	sell := func(p string) *trade.Order {
		return &trade.Order{Symbol: "AAPL", Side: trade.Sell, Type: trade.Limit, Quantity: trade.Price("1"), LimitPrice: trade.Price(p)}
	}
	for name, combo := range map[string]*trade.Combo{
		"oto":   trade.OTO(buy("1.00"), sell("999")),
		"oco":   trade.OCO(buy("1.00"), buy("1.01")),
		"otoco": trade.OTOCO(buy("1.00"), sell("999"), sell("998")),
	} {
		t.Run(name, func(t *testing.T) {
			err := retryTransient(ctx, t, "PreviewCombo", func() error {
				_, e := c.PreviewCombo(ctx, acct.AccountID, combo)
				return e
			})
			if err == nil {
				t.Errorf("%s: the sandbox now accepts this group; update the compatibility matrix and backlog item 17", name)
				return
			}
			var apiErr *webull.APIError
			if errors.As(err, &apiErr) && apiErr.Code == "OPENAPI_PARAM_ERR" && strings.Contains(apiErr.Message, "combo_type") {
				t.Skipf("integration: sandbox rejects %s groups: %s", name, apiErr.Message)
			}
			t.Fatalf("%s: unexpected error: %v", name, err)
		})
	}
}

func accountOfClass(ctx context.Context, t *testing.T, c *trade.Client, class trade.AccountClass) trade.Account {
	t.Helper()
	return accountWhere(ctx, t, c, string(class)+" account", func(a trade.Account) bool { return a.AccountClass == class })
}

// skipIfCode skips the test when err carries one of the given Webull codes,
// reporting the message, so that a sandbox limitation is visible rather than
// silently green.
func skipIfCode(t *testing.T, err error, codes ...string) {
	t.Helper()
	var apiErr *webull.APIError
	if errors.As(err, &apiErr) {
		for _, code := range codes {
			if apiErr.Code == code {
				t.Skipf("integration: sandbox: %s (%s)", apiErr.Message, apiErr.Code)
			}
		}
	}
}

// marketClosedCodes are the codes Webull returns when an order cannot be
// placed because the market is closed. They are not transient within a test
// run, so the test skips rather than retries.
var marketClosedCodes = []string{
	"OPENAPI_DAY_ORDER_NOT_ALLOWED_AFT_CORE_TIME_LIMIT",     // a DAY order outside regular hours
	"OPENAPI_FUTURES_CAN_NOT_TRADING_FOR_NON_TRADING_HOURS", // futures outside their session
	"OPENAPI_CAN_NOT_TRADING_FOR_NON_TRADING_HOURS",         // equities outside 9:30–16:00 ET (observed on combos)
	"OPENAPI_CAN_NOT_TRADING_FOR_FIXGW_NOT_READY",           // the order gateway is not accepting orders now
}

// skipIfMarketClosed skips the test when err says an order cannot be placed
// because the market is closed.
func skipIfMarketClosed(t *testing.T, err error) {
	t.Helper()
	skipIfCode(t, err, marketClosedCodes...)
}

// isTransient reports whether err is a sandbox condition that clears on its
// own: rate limiting under load, or a just-placed order not yet in a
// cancellable state.
func isTransient(err error) bool {
	if errors.Is(err, webull.ErrRateLimited) {
		return true
	}
	var apiErr *webull.APIError
	return errors.As(err, &apiErr) && apiErr.Code == "OPENAPI_ORDER_CAN_NOT_BE_CANCEL_FOR_PENDING_SUBMIT"
}

// retryTransient runs fn until it succeeds, fails non-transiently, or the
// attempts are exhausted, backing off between tries. It is for integration
// calls that are safe to repeat: reads, previews, cancels, and placements
// (a rate-limited request is rejected at the gateway before an order is
// created, so repeating it cannot double-place).
func retryTransient(ctx context.Context, t *testing.T, what string, fn func() error) error {
	t.Helper()
	var err error
	for attempt := 0; attempt < 6; attempt++ {
		if err = fn(); err == nil || !isTransient(err) {
			return err
		}
		delay := time.Duration(attempt+1) * time.Second
		t.Logf("integration: %s hit a transient condition, retrying in %s: %v", what, delay, err)
		select {
		case <-ctx.Done():
			return err
		case <-time.After(delay):
		}
	}
	return err
}

// readOrder reads an order's group, riding out a rate-limit burst under
// full-suite load by backing off until the read succeeds or the context
// ends. It returns a non-transient error unchanged, so a poller sees order
// state rather than a load-induced failure.
func readOrder(ctx context.Context, c *trade.Client, accountID, clientOrderID string) (*trade.OrderGroup, error) {
	for {
		g, err := c.Order(ctx, accountID, clientOrderID)
		if err == nil || !isTransient(err) {
			return g, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// previewOrder previews an order, riding out a rate-limit burst so a test
// sees the server's real answer — a preview or a genuine rejection — rather
// than a load-induced 429.
func previewOrder(ctx context.Context, t *testing.T, c *trade.Client, accountID string, o *trade.Order) (*trade.OrderPreview, error) {
	t.Helper()
	var p *trade.OrderPreview
	err := retryTransient(ctx, t, "PreviewOrder", func() error {
		var e error
		p, e = c.PreviewOrder(ctx, accountID, o)
		return e
	})
	return p, err
}

// restingLifecycle places o, confirms it is working, cancels it and confirms
// the cancellation. o must be priced so that it cannot fill. Order state is
// polled rather than assumed after a fixed delay.
func restingLifecycle(ctx context.Context, t *testing.T, c *trade.Client, acct trade.Account, o *trade.Order) {
	t.Helper()
	var receipt *trade.OrderReceipt
	err := retryTransient(ctx, t, "PlaceOrder", func() error {
		var e error
		receipt, e = c.PlaceOrder(ctx, acct.AccountID, o)
		return e
	})
	if err != nil {
		skipIfMarketClosed(t, err)
		t.Fatalf("PlaceOrder: %v", err)
	}
	// The cleanup runs after ctx has been cancelled, so it needs its own.
	t.Cleanup(func() { _, _ = c.CancelOrder(context.Background(), acct.AccountID, o.ClientOrderID) }) //nolint:contextcheck // see above
	if receipt.OrderID == "" {
		t.Fatalf("receipt = %+v", receipt)
	}

	poll := func(what string, pred func(*trade.OrderGroup) bool) {
		t.Helper()
		var last *trade.OrderGroup
		for {
			g, err := readOrder(ctx, c, acct.AccountID, o.ClientOrderID)
			if err != nil {
				t.Fatalf("order never %s: %v; last = %+v", what, err, last)
			}
			last = g
			if pred(g) {
				return
			}
			// A fill is terminal: the account now holds a position, and the
			// order will never become working or cancelled.
			if len(g.Orders) == 1 && g.Orders[0].Status == trade.StatusFilled {
				t.Fatalf("order filled instead of resting; the sandbox %s account now holds %s %s", acct.AccountClass, o.Symbol, g.Orders[0].TotalQuantity)
			}
			select {
			case <-ctx.Done():
				t.Fatalf("order never %s; last = %+v", what, last)
			case <-time.After(500 * time.Millisecond):
			}
		}
	}

	poll("working", func(g *trade.OrderGroup) bool {
		return len(g.Orders) == 1 && g.Orders[0].InstrumentType == o.InstrumentType &&
			(g.Orders[0].Status == trade.StatusSubmitted || g.Orders[0].Status == trade.StatusPending)
	})
	// A just-placed order can briefly report "pending submit" and refuse a
	// cancel; retryTransient rides that out.
	if err := retryTransient(ctx, t, "CancelOrder", func() error {
		_, e := c.CancelOrder(ctx, acct.AccountID, o.ClientOrderID)
		return e
	}); err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	poll("cancelled", func(g *trade.OrderGroup) bool {
		return len(g.Orders) == 1 && g.Orders[0].Status == trade.StatusCancelled
	})
}

func TestIntegrationFuturesOrder(t *testing.T) {
	c, ctx := newClient(t)
	acct := accountOfClass(ctx, t, c, trade.AccountClassFutures)
	contracts, err := c.FuturesContracts(ctx, trade.FuturesContractsRequest{Code: "ES"})
	if err != nil || len(contracts) == 0 {
		t.Fatalf("FuturesContracts: %v (%d)", err, len(contracts))
	}
	o := &trade.Order{InstrumentType: trade.InstrumentFutures, Symbol: contracts[0].Symbol, Side: trade.Buy, Type: trade.Limit,
		TimeInForce: trade.GTC, Quantity: trade.Price("1"), LimitPrice: trade.Price("1.00")}
	if _, err := previewOrder(ctx, t, c, acct.AccountID, o); err != nil {
		t.Fatalf("PreviewOrder: %v", err)
	}
	// Futures do not trade during the exchange's daily break; the lifecycle
	// skips with Webull's reason when that is the case.
	restingLifecycle(ctx, t, c, acct, o)
}

func TestIntegrationCryptoOrder(t *testing.T) {
	c, ctx := newClient(t)
	acct := accountOfClass(ctx, t, c, trade.AccountClassCrypto)
	// $5 notional: Webull's minimum is $2.
	o := &trade.Order{InstrumentType: trade.InstrumentCrypto, Symbol: "BTCUSD", Side: trade.Buy, Type: trade.Limit,
		TimeInForce: trade.GTC, Quantity: trade.Price("0.001"), LimitPrice: trade.Price("5000")}

	// The sandbox answers every crypto preview with a system error while
	// accepting the same order for placement. Record it rather than fail.
	if _, err := previewOrder(ctx, t, c, acct.AccountID, o); err != nil {
		var apiErr *webull.APIError
		if errors.As(err, &apiErr) && apiErr.Code == "OPENAPI_SYSTEM_ERROR" {
			t.Logf("integration: sandbox rejects crypto previews with %s; placement is verified below", apiErr.Code)
		} else {
			t.Fatalf("PreviewOrder: %v", err)
		}
	} else {
		t.Log("the sandbox now accepts crypto previews; update the compatibility matrix")
	}
	restingLifecycle(ctx, t, c, acct, o)
}

func TestIntegrationEventContractOrder(t *testing.T) {
	c, ctx := newClient(t)
	acct := accountOfClass(ctx, t, c, trade.AccountClassEventsCash)
	markets, err := c.EventMarkets(ctx, trade.EventMarketsRequest{SeriesSymbol: "KXRATECUTCOUNT"})
	if err != nil || len(markets.Markets) == 0 {
		t.Fatalf("EventMarkets: %v", err)
	}
	var symbol string
	for _, m := range markets.Markets {
		if m.TradableStatus == trade.Tradable {
			symbol = m.Symbol
			break
		}
	}
	if symbol == "" {
		t.Skip("integration: no tradable event market in the series")
	}
	for _, tif := range []trade.TimeInForce{trade.Day, trade.GTC} {
		t.Run(string(tif), func(t *testing.T) {
			ctx := testutil.IntegrationContext(t) // each lifecycle gets its own budget
			o := &trade.Order{InstrumentType: trade.InstrumentEvent, Symbol: symbol, Side: trade.Buy, Type: trade.Limit,
				TimeInForce: tif, Quantity: trade.Price("1"), LimitPrice: trade.Price("0.01"), EventOutcome: trade.OutcomeYes}
			if err := retryTransient(ctx, t, "PreviewOrder", func() error {
				_, e := c.PreviewOrder(ctx, acct.AccountID, o)
				return e
			}); err != nil {
				t.Fatalf("PreviewOrder: %v", err)
			}
			restingLifecycle(ctx, t, c, acct, o)
		})
	}
}

func TestIntegrationBatchPlace(t *testing.T) {
	c, ctx := newClient(t)
	acct := marginAccount(ctx, t, c)
	orders := []*trade.Order{
		{Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit, Quantity: trade.Price("1"), LimitPrice: trade.Price("1.00")},
		{Symbol: "MSFT", Side: trade.Buy, Type: trade.Limit, Quantity: trade.Price("1"), LimitPrice: trade.Price("1.00")},
	}
	result, err := c.PlaceOrders(ctx, acct.AccountID, orders)
	if err != nil {
		// Batches are DAY only, so outside regular hours this cannot run;
		// and the sandbox account is not enabled for batch placement at all.
		skipIfMarketClosed(t, err)
		var apiErr *webull.APIError
		if errors.As(err, &apiErr) && strings.Contains(apiErr.Message, "Account not supported") {
			t.Skipf("integration: sandbox: batch placement is not enabled for this account: %s", apiErr.Message)
		}
		t.Fatalf("PlaceOrders: %v", err)
	}
	t.Cleanup(func() {
		for _, o := range orders {
			_, _ = c.CancelOrder(context.Background(), acct.AccountID, o.ClientOrderID)
		}
	})
	if result.Total != 2 {
		t.Errorf("result = %+v", result)
	}
	for _, r := range result.Orders {
		if r.Code != "" {
			t.Errorf("order %s rejected: %s %s", r.ClientOrderID, r.Code, r.Message)
		}
	}
}
