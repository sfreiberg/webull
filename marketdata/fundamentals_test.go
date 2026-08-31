package marketdata

import (
	"context"
	"testing"
	"time"
)

func TestBalanceSheetsRequestAndDecoding(t *testing.T) {
	c, f := newClient(t, "/market-data/fundamentals/balance-sheets/get", "balance_sheets.json", 0)
	got, err := c.BalanceSheets(context.Background(), FinancialsRequest{Symbol: "AAPL", Type: Annual, Count: 4})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["category"][0] != "US_STOCK" || f.gotQuery["type"][0] != "ANNUAL" || f.gotQuery["count"][0] != "4" {
		t.Errorf("query = %v", f.gotQuery)
	}
	bs := got[0]
	if bs.FiscalYear != 2026 || bs.FiscalPeriod != 0 || bs.Currency != "USD" || bs.EndDate.IsZero() || bs.PublishDate.IsZero() {
		t.Errorf("period = %+v", bs.ReportPeriod)
	}
	if !bs.TotalAssets.Decimal.Equal(d("379297000000")) || !bs.RetainedEarnings.Decimal.Equal(d("-2177000000")) || !bs.CommonSharesOutstanding.Decimal.Equal(d("14702703000")) {
		t.Errorf("balance sheet = %+v", bs)
	}
	if bs.GoodwillNet.Valid {
		t.Error("an absent line item must be null, not zero")
	}
}

func TestFinancialsOmitTypeAndCountWhenUnset(t *testing.T) {
	c, f := newClient(t, "", "balance_sheets.json", 0)
	if _, err := c.BalanceSheets(context.Background(), FinancialsRequest{Symbol: "AAPL"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.gotQuery["type"]; ok {
		t.Errorf("type must be omitted when unset: %v", f.gotQuery)
	}
	if _, ok := f.gotQuery["count"]; ok {
		t.Errorf("count must be omitted when unset: %v", f.gotQuery)
	}
}

func TestIncomeStatements(t *testing.T) {
	c, _ := newClient(t, "/market-data/fundamentals/income-statements/get", "income_statements.json", 0)
	got, err := c.IncomeStatements(context.Background(), FinancialsRequest{Symbol: "AAPL"})
	if err != nil {
		t.Fatal(err)
	}
	is := got[0]
	if !is.TotalRevenue.Decimal.Equal(d("416161000000")) || !is.DilutedEPSInclExtra.Decimal.Equal(d("7.464996")) || !is.DividendsPerShare.Decimal.Equal(d("1.02")) {
		t.Errorf("income statement = %+v", is)
	}
	if is.MinorityInterest.Valid {
		t.Error("an absent line item must be null, not zero")
	}
}

func TestCashFlows(t *testing.T) {
	c, _ := newClient(t, "/market-data/fundamentals/cash-flows/get", "cash_flows.json", 0)
	got, err := c.CashFlows(context.Background(), FinancialsRequest{Symbol: "AAPL"})
	if err != nil {
		t.Fatal(err)
	}
	cf := got[0]
	if !cf.OperatingCashFlow.Decimal.Equal(d("14747000000")) || !cf.CapitalExpenditures.Decimal.Equal(d("-8527000000")) || !cf.NetChangeInCash.Decimal.Equal(d("579000000")) {
		t.Errorf("cash flow = %+v", cf)
	}
}

func TestFinancialIndicators(t *testing.T) {
	c, _ := newClient(t, "/market-data/fundamentals/indicators/get", "financial_indicators.json", 0)
	got, err := c.FinancialIndicators(context.Background(), FinancialsRequest{Symbol: "AAPL"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Currency != "USD" || len(got.Values["roa"]) != 2 {
		t.Errorf("indicators = %+v", got)
	}
	roa := got.Values["roa"][0]
	if roa.FiscalYear != 2025 || roa.FiscalPeriod != 4 || !roa.Value.Decimal.Equal(d("0.2133")) {
		t.Errorf("roa = %+v", roa)
	}
	if got.Values["roe"][0].Value.Valid {
		t.Error("a null indicator value must be null, not zero")
	}
}

func TestFinancialAlert(t *testing.T) {
	c, f := newClient(t, "/market-data/fundamentals/financial-alerts/get", "financial_alert.json", 0)
	got, err := c.FinancialAlert(context.Background(), "AAPL", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["category"][0] != "US_STOCK" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if got.FiscalYear != 2026 || got.FiscalPeriod != 1 || got.StartDate.IsZero() || !got.EPSEstimate.Decimal.Equal(d("1.9439")) {
		t.Errorf("alert = %+v", got)
	}
}

func TestCapitalFlowsDecodeScientificNotationAndCompactDates(t *testing.T) {
	c, f := newClient(t, "/market-data/fundamentals/capital-flows/get", "capital_flows.json", 0)
	got, err := c.CapitalFlows(context.Background(), "AAPL", "", 5)
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["count"][0] != "5" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if len(got) != 2 || !got[1].Date.Equal(time.Date(2026, 5, 22, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("flows = %+v", got)
	}
	if !got[1].LargeIn.Equal(d("178524330.946")) || !got[1].SmallOut.Equal(d("544792669.0503")) {
		t.Errorf("scientific notation did not decode: %+v", got[1])
	}
}

func TestDividendCalendar(t *testing.T) {
	c, _ := newClient(t, "/market-data/fundamentals/dividend-calendars/list", "dividend_calendar.json", 0)
	got, err := c.DividendCalendar(context.Background(), "AAPL", "")
	if err != nil {
		t.Fatal(err)
	}
	dv := got[0]
	if dv.Type != "CASH_DIVIDEND" || !dv.Amount.Equal(d("0.25")) || dv.ExDividendDate.IsZero() || dv.PayDate.IsZero() {
		t.Errorf("dividend = %+v", dv)
	}
}

func TestEarningsCalendar(t *testing.T) {
	c, _ := newClient(t, "/market-data/fundamentals/earnings-calendars/list", "earnings_calendar.json", 0)
	got, err := c.EarningsCalendar(context.Background(), "AAPL", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].EPSActual.Decimal.Equal(d("2.18")) || got[0].ExpectedPublishDate.IsZero() {
		t.Errorf("earnings = %+v", got)
	}
	if got[1].EPSActual.Valid {
		t.Error("an unreported quarter's actual EPS must be null")
	}
}

func TestFilingsUnwrapTheEnvelope(t *testing.T) {
	c, _ := newClient(t, "/market-data/fundamentals/filings/list", "filings.json", 0)
	got, err := c.Filings(context.Background(), "AAPL", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title == "" || got[0].URL == "" || got[0].PublishDate.IsZero() {
		t.Errorf("filings = %+v", got)
	}
}

func TestForecastEPS(t *testing.T) {
	c, _ := newClient(t, "/market-data/fundamentals/forecast-eps/get", "forecast_eps.json", 0)
	got, err := c.ForecastEPS(context.Background(), "AAPL", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Reported || !got[0].Actual.Decimal.Equal(d("-0.02")) {
		t.Errorf("forecast = %+v", got)
	}
	if got[1].Reported || got[1].Actual.Valid {
		t.Errorf("an unreported quarter must have no actual: %+v", got[1])
	}
}

func TestIndustryComparison(t *testing.T) {
	c, f := newClient(t, "/market-data/fundamentals/industry-comparisons/get", "industry_comparison.json", 0)
	got, err := c.IndustryComparison(context.Background(), "AAPL", "", EPSTTM)
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["sort_by"][0] != "EPS_TTM" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if got.IndustryName != "Phones & Handheld Devices" || got.Metric != EPSTTM || len(got.Companies) != 3 {
		t.Errorf("comparison = %+v", got)
	}
	if got.Companies[0].Rank != 1 || !got.Companies[0].Value.Decimal.Equal(d("8.266")) {
		t.Errorf("entry = %+v", got.Companies[0])
	}
	if got.Companies[2].Value.Valid {
		t.Error("a null metric value must be null, not zero")
	}
}

func TestIndustryComparisonOmitsMetricWhenUnset(t *testing.T) {
	c, f := newClient(t, "", "industry_comparison.json", 0)
	if _, err := c.IndustryComparison(context.Background(), "AAPL", "", ""); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.gotQuery["sort_by"]; ok {
		t.Errorf("sort_by must be omitted when unset: %v", f.gotQuery)
	}
}

func TestErrorsPropagateFromEveryFundamentalsMethod(t *testing.T) {
	assertCategoryErrors(t, func(c *Client, ctx context.Context) map[string]func() error {
		return map[string]func() error{
			"BalanceSheets":       func() error { _, e := c.BalanceSheets(ctx, FinancialsRequest{Symbol: "A"}); return e },
			"IncomeStatements":    func() error { _, e := c.IncomeStatements(ctx, FinancialsRequest{Symbol: "A"}); return e },
			"CashFlows":           func() error { _, e := c.CashFlows(ctx, FinancialsRequest{Symbol: "A"}); return e },
			"FinancialIndicators": func() error { _, e := c.FinancialIndicators(ctx, FinancialsRequest{Symbol: "A"}); return e },
			"FinancialAlert":      func() error { _, e := c.FinancialAlert(ctx, "A", ""); return e },
			"CapitalFlows":        func() error { _, e := c.CapitalFlows(ctx, "A", "", 0); return e },
			"DividendCalendar":    func() error { _, e := c.DividendCalendar(ctx, "A", ""); return e },
			"EarningsCalendar":    func() error { _, e := c.EarningsCalendar(ctx, "A", ""); return e },
			"Filings":             func() error { _, e := c.Filings(ctx, "A", ""); return e },
			"ForecastEPS":         func() error { _, e := c.ForecastEPS(ctx, "A", ""); return e },
			"IndustryComparison":  func() error { _, e := c.IndustryComparison(ctx, "A", "", ""); return e },
		}
	})
}
