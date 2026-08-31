package marketdata

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/sfreiberg/webull/internal/query"
)

// CompanyProfile describes a listed company.
type CompanyProfile struct {
	Symbol        string   `json:"symbol"`
	Category      Category `json:"category"`
	CompanyName   string   `json:"company_name"`
	EstablishDate string   `json:"establish_date"`
	Exchange      string   `json:"exhibition_code"`
	Profile       string   `json:"profile"`
	Employees     string   `json:"employees"`
	Address       string   `json:"address"`
	CEO           string   `json:"ceo"`
	Industries    []string `json:"industries"`
}

// CompanyProfile returns the profile of a listed company.
func (c *Client) CompanyProfile(ctx context.Context, symbol string, cat Category) (*CompanyProfile, error) {
	var out CompanyProfile
	if err := c.get(ctx, "/market-data/fundamentals/company-profiles/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AnalystRating is the consensus of analyst recommendations.
type AnalystRating struct {
	Symbol        string          `json:"symbol"`
	Category      Category        `json:"category"`
	Analysts      decimal.Decimal `json:"number"`
	StrongBuy     decimal.Decimal `json:"strong_buy"`
	Buy           decimal.Decimal `json:"buy"`
	Hold          decimal.Decimal `json:"hold"`
	UnderPerform  decimal.Decimal `json:"under_perform"`
	Sell          decimal.Decimal `json:"sell"`
	EffectiveFrom Time            `json:"effective_start_date"`
}

// AnalystRating returns the consensus analyst rating for a stock.
func (c *Client) AnalystRating(ctx context.Context, symbol string, cat Category) (*AnalystRating, error) {
	var out AnalystRating
	if err := c.get(ctx, "/market-data/fundamentals/analysis/ratings/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TargetPrice is the consensus of analyst price targets.
type TargetPrice struct {
	Symbol        string          `json:"symbol"`
	Category      Category        `json:"category"`
	Mean          decimal.Decimal `json:"mean"`
	Median        decimal.Decimal `json:"median"`
	Low           decimal.Decimal `json:"low"`
	High          decimal.Decimal `json:"high"`
	Currency      string          `json:"currency"`
	EffectiveFrom Time            `json:"effective_start_date"`
}

// TargetPrice returns the consensus analyst price target for a stock.
func (c *Client) TargetPrice(ctx context.Context, symbol string, cat Category) (*TargetPrice, error) {
	var out TargetPrice
	if err := c.get(ctx, "/market-data/fundamentals/analysis/target-prices/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CapitalFlow is one trading day's traded value broken down by order size,
// in the market's base currency.
type CapitalFlow struct {
	Date      Time            `json:"date"`
	LargeIn   decimal.Decimal `json:"large_in"`
	LargeOut  decimal.Decimal `json:"large_out"`
	MediumIn  decimal.Decimal `json:"medium_in"`
	MediumOut decimal.Decimal `json:"medium_out"`
	SmallIn   decimal.Decimal `json:"small_in"`
	SmallOut  decimal.Decimal `json:"small_out"`
}

// CapitalFlows returns the most recent trading days' capital flow, oldest
// first. Count is between 1 and 5; zero lets Webull apply its default of 5.
func (c *Client) CapitalFlows(ctx context.Context, symbol string, cat Category, count int) ([]CapitalFlow, error) {
	q := symbolParams(symbol, cat)
	q.SetInt("count", count)
	var out []CapitalFlow
	if err := c.get(ctx, "/market-data/fundamentals/capital-flows/get", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Dividend is one declared dividend.
type Dividend struct {
	Symbol         string          `json:"symbol"`
	Market         string          `json:"market"`
	Currency       string          `json:"currency"`
	Amount         decimal.Decimal `json:"amount"`
	Type           string          `json:"div_type"`
	DeclareDate    Time            `json:"declare_date"`
	ExDividendDate Time            `json:"ex_div_date"`
	RecordDate     Time            `json:"record_date"`
	PayDate        Time            `json:"pay_date"`
}

// DividendCalendar returns a stock's declared dividends.
func (c *Client) DividendCalendar(ctx context.Context, symbol string, cat Category) ([]Dividend, error) {
	var out []Dividend
	if err := c.get(ctx, "/market-data/fundamentals/dividend-calendars/list", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Earnings is one quarter's estimated and, once reported, actual earnings.
type Earnings struct {
	FiscalYear          int                 `json:"fiscal_year"`
	FiscalPeriod        int                 `json:"fiscal_period"`
	Currency            string              `json:"currency"`
	ExpectedPublishDate Time                `json:"expected_publish_date"`
	EPSActual           decimal.NullDecimal `json:"eps_actual"`
	EPSEstimate         decimal.NullDecimal `json:"eps_est"`
	RevenueActual       decimal.NullDecimal `json:"rev_actual"`
	RevenueEstimate     decimal.NullDecimal `json:"rev_est"`
}

// EarningsCalendar returns a stock's quarterly earnings dates and estimates.
func (c *Client) EarningsCalendar(ctx context.Context, symbol string, cat Category) ([]Earnings, error) {
	var out []Earnings
	if err := c.get(ctx, "/market-data/fundamentals/earnings-calendars/list", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Filing is one regulatory filing.
type Filing struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	PublishDate Time   `json:"publish_date"`
}

// Filings returns a company's regulatory filings.
func (c *Client) Filings(ctx context.Context, symbol string, cat Category) ([]Filing, error) {
	var out struct {
		Filings []Filing `json:"filings"`
	}
	if err := c.get(ctx, "/market-data/fundamentals/filings/list", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return out.Filings, nil
}

// EPSForecast is one quarter's estimated and, once reported, actual EPS.
type EPSForecast struct {
	FiscalYear   int                 `json:"fiscal_year"`
	FiscalPeriod int                 `json:"fiscal_period"`
	Actual       decimal.NullDecimal `json:"actual"`
	Estimate     decimal.NullDecimal `json:"est"`
	Reported     bool                `json:"reported"`
}

// ForecastEPS returns a stock's quarterly EPS estimates and results.
func (c *Client) ForecastEPS(ctx context.Context, symbol string, cat Category) ([]EPSForecast, error) {
	var out []EPSForecast
	if err := c.get(ctx, "/market-data/fundamentals/forecast-eps/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// IndustryRank is one company's rank within its industry on the compared
// metric.
type IndustryRank struct {
	Symbol string          `json:"symbol"`
	Name   string          `json:"name"`
	Rank   int             `json:"rank"`
	Value  decimal.Decimal `json:"value"`
}

// IndustryComparison ranks a company's industry peers on one metric.
type IndustryComparison struct {
	FiscalYear   int            `json:"fiscal_year"`
	FiscalPeriod int            `json:"fiscal_period"`
	IndustryName string         `json:"industry_name"`
	Metric       string         `json:"type"`
	Companies    []IndustryRank `json:"data"`
}

// IndustryComparison compares a company with its industry peers on one
// financial metric; an empty metric lets Webull default to "EPS_TTM".
func (c *Client) IndustryComparison(ctx context.Context, symbol string, cat Category, metric string) (*IndustryComparison, error) {
	q := symbolParams(symbol, cat)
	q.Set("sort_by", metric)
	var out IndustryComparison
	if err := c.get(ctx, "/market-data/fundamentals/industry-comparisons/get", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func symbolParams(symbol string, cat Category) query.Params {
	q := query.New()
	q.Set("symbol", symbol)
	q.Set("category", string(category(cat)))
	return q
}
