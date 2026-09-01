package marketdata

import (
	"context"
	"testing"
	"time"
)

func TestFundBrief(t *testing.T) {
	c, f := newClient(t, "/market-data/fundamentals/fund-brief/get", "fund_brief.json", 0)
	got, err := c.FundBrief(context.Background(), "SPY", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["category"][0] != "US_STOCK" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if got.Name == "" || got.LaunchDate.IsZero() || !got.AUM.Decimal.Equal(d("811937033817.00")) || len(got.Managers) != 2 {
		t.Errorf("brief = %+v", got)
	}
	if got.Managers[0].Incumbent != 0 || got.Managers[1].Incumbent != 1 {
		t.Errorf("incumbent flags = %+v", got.Managers)
	}
	if got.Managers[1].TenureReturn.Valid {
		t.Error("an absent tenure return must be null, not zero")
	}
}

func TestFundAllocationsDecodeSparseAssets(t *testing.T) {
	c, _ := newClient(t, "/market-data/fundamentals/fund-allocations/get", "fund_allocations.json", 0)
	got, err := c.FundAllocations(context.Background(), "SPY", "")
	if err != nil {
		t.Fatal(err)
	}
	a := got[0]
	if a.Date.IsZero() || !a.AUM.Decimal.Equal(d("422219392815")) || !a.Stock.Ratio.Decimal.Equal(d("99.9174")) {
		t.Errorf("allocation = %+v", a)
	}
	if a.Bond.Value.Valid {
		t.Error("an empty asset class must stay null")
	}
}

func TestFundDividendsPaginate(t *testing.T) {
	c, f := newClient(t, "/market-data/fundamentals/fund-dividends/get", "fund_dividends.json", 0)
	got, err := c.FundDividends(context.Background(), "SPY", "", "abc")
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["pagination_key"][0] != "abc" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if len(got.Dividends) != 1 || got.Dividends[0].ExDividendDate.IsZero() || !got.Dividends[0].PerShare.Equal(d("0.071616")) {
		t.Errorf("dividends = %+v", got.Dividends)
	}
	if got.PaginationKey == "" {
		t.Error("pagination key did not decode")
	}
}

func TestFundFilesHoldingsNetValuesRatingsSplits(t *testing.T) {
	c, _ := newClient(t, "/market-data/fundamentals/fund-files/get", "fund_files.json", 0)
	files, err := c.FundFiles(context.Background(), "SPY", "")
	if err != nil || len(files) != 1 || files[0].Type != 1 || files[0].URL == "" || files[0].PublishDate.IsZero() {
		t.Errorf("FundFiles: %v %+v", err, files)
	}

	c2, _ := newClient(t, "/market-data/fundamentals/fund-holdings/get", "fund_holdings.json", 0)
	holdings, err := c2.FundHoldings(context.Background(), "SPY", "")
	if err != nil || len(holdings) != 2 || !holdings[0].HeldPercent.Decimal.Equal(d("8.91904")) {
		t.Fatalf("FundHoldings: %v %+v", err, holdings)
	}
	if !holdings[0].MaturityDate.IsZero() || holdings[1].MaturityDate.IsZero() {
		t.Errorf("maturity dates = %+v", holdings)
	}
	if holdings[1].HeldChangePercent.Valid {
		t.Error("an absent change percent must be null")
	}

	c3, f3 := newClient(t, "/market-data/fundamentals/fund-net-values/get", "fund_net_values.json", 0)
	navs, err := c3.FundNetValues(context.Background(), "SPY", "", time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC), 2)
	if err != nil || len(navs) != 2 || !navs[0].NetValue.Equal(d("766.859415")) {
		t.Errorf("FundNetValues: %v %+v", err, navs)
	}
	if f3.gotQuery["last_date"][0] != "2026-08-31" || f3.gotQuery["count"][0] != "2" {
		t.Errorf("query = %v", f3.gotQuery)
	}

	c4, f4 := newClient(t, "/market-data/fundamentals/fund-net-values/get", "fund_net_values.json", 0)
	if _, err := c4.FundNetValues(context.Background(), "SPY", "", time.Time{}, 0); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"last_date", "count"} {
		if _, ok := f4.gotQuery[k]; ok {
			t.Errorf("%s must be omitted when unset: %v", k, f4.gotQuery)
		}
	}

	c5, _ := newClient(t, "/market-data/fundamentals/fund-ratings/get", "fund_ratings.json", 0)
	ratings, err := c5.FundRatings(context.Background(), "SPY", "")
	if err != nil || len(ratings) != 1 || ratings[0].Rating != 5 || ratings[0].Agency != "Morningstar" {
		t.Errorf("FundRatings: %v %+v", err, ratings)
	}

	c6, _ := newClient(t, "/market-data/fundamentals/fund-splits/get", "fund_splits.json", 0)
	splits, err := c6.FundSplits(context.Background(), "SPY", "")
	if err != nil || len(splits) != 1 || splits[0].Type != "MERGE" || !splits[0].From.Equal(d("1")) || !splits[0].To.Equal(d("2")) {
		t.Errorf("FundSplits: %v %+v", err, splits)
	}
}

func TestFundPerformance(t *testing.T) {
	c, _ := newClient(t, "/market-data/fundamentals/fund-performances/get", "fund_performance.json", 0)
	got, err := c.FundPerformance(context.Background(), "SPY", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Currency != "USD" || got.EndDate.IsZero() || !got.Return1M.Decimal.Equal(d("4.78693")) || !got.SinceInception.Decimal.Equal(d("512.6")) {
		t.Errorf("performance = %+v", got)
	}
	if got.Return10Y.Valid {
		t.Error("an absent trailing return must be null, not zero")
	}
}

func TestErrorsPropagateFromEveryFundMethod(t *testing.T) {
	assertCategoryErrors(t, func(c *Client, ctx context.Context) map[string]func() error {
		return map[string]func() error{
			"FundBrief":       func() error { _, e := c.FundBrief(ctx, "A", ""); return e },
			"FundAllocations": func() error { _, e := c.FundAllocations(ctx, "A", ""); return e },
			"FundDividends":   func() error { _, e := c.FundDividends(ctx, "A", "", ""); return e },
			"FundFiles":       func() error { _, e := c.FundFiles(ctx, "A", ""); return e },
			"FundHoldings":    func() error { _, e := c.FundHoldings(ctx, "A", ""); return e },
			"FundNetValues":   func() error { _, e := c.FundNetValues(ctx, "A", "", time.Time{}, 0); return e },
			"FundPerformance": func() error { _, e := c.FundPerformance(ctx, "A", ""); return e },
			"FundRatings":     func() error { _, e := c.FundRatings(ctx, "A", ""); return e },
			"FundSplits":      func() error { _, e := c.FundSplits(ctx, "A", ""); return e },
		}
	})
}
