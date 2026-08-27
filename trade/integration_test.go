package trade_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/trade"
)

// These run against the live sandbox whenever credentials are present and
// skip with a reported reason otherwise. Everything here is read-only.

func newClient(t *testing.T) (*trade.Client, context.Context) {
	t.Helper()
	key, secret := os.Getenv("WEBULL_APP_KEY"), os.Getenv("WEBULL_APP_SECRET")
	if key == "" || secret == "" {
		t.Skip("integration: WEBULL_APP_KEY and WEBULL_APP_SECRET are not set")
	}
	c, err := webull.NewClient(webull.Config{AppKey: key, AppSecret: secret, Environment: webull.Sandbox})
	if err != nil {
		t.Fatal(err)
	}
	if c.Environment().IsProduction() {
		t.Fatal("integration tests must never run against production")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return c.Trade, ctx
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
		for _, k := range page.Contracts[:5] {
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
