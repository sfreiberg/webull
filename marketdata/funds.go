package marketdata

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// FundBrief describes a fund: objective, benchmark, size and management.
type FundBrief struct {
	Name                string              `json:"name"`
	LaunchDate          Time                `json:"launch_date"`
	Benchmark           string              `json:"benchmark"`
	InvestmentObjective string              `json:"investment_objective"`
	AUM                 decimal.NullDecimal `json:"aum"`
	Issuer              string              `json:"issuer"`
	Custodian           string              `json:"custodian"`
	Managers            []FundManager       `json:"managers"`
}

// FundManager is one manager's tenure at a fund.
type FundManager struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	StartDate Time   `json:"start_date"`
	EndDate   Time   `json:"end_date"`
	// Incumbent is 1 while the manager is in post.
	Incumbent    int                 `json:"is_incumbent"`
	TenureReturn decimal.NullDecimal `json:"tenure_return"`
	TenureYears  decimal.NullDecimal `json:"tenure_years"`
	TenureDays   int                 `json:"tenure_days"`
}

// FundBrief returns a fund's profile.
func (c *Client) FundBrief(ctx context.Context, symbol string, cat Category) (*FundBrief, error) {
	var out FundBrief
	if err := c.get(ctx, "/market-data/fundamentals/fund-brief/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FundAsset is one asset class's share of a fund's portfolio.
type FundAsset struct {
	Value decimal.NullDecimal `json:"value"`
	Ratio decimal.NullDecimal `json:"ratio"`
}

// FundAllocation is a fund's portfolio broken down by asset class on one
// reporting date.
type FundAllocation struct {
	Date        Time                `json:"date"`
	AUM         decimal.NullDecimal `json:"aum"`
	Cash        FundAsset           `json:"cash"`
	Bond        FundAsset           `json:"bond"`
	Stock       FundAsset           `json:"stock"`
	Preferred   FundAsset           `json:"preferred"`
	Convertible FundAsset           `json:"convertible"`
	Other       FundAsset           `json:"other"`
}

// FundAllocations returns a fund's asset allocation history.
func (c *Client) FundAllocations(ctx context.Context, symbol string, cat Category) ([]FundAllocation, error) {
	var out []FundAllocation
	if err := c.get(ctx, "/market-data/fundamentals/fund-allocations/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FundDividend is one distribution by a fund.
type FundDividend struct {
	DeclareDate    Time            `json:"publish_date"`
	ExDividendDate Time            `json:"share_date"`
	RecordDate     Time            `json:"record_date"`
	PayDate        Time            `json:"pay_date"`
	PerShare       decimal.Decimal `json:"dps"`
}

// FundDividendsPage is one page of fund distributions.
type FundDividendsPage struct {
	Dividends []FundDividend `json:"data"`
	// PaginationKey continues the listing; empty on the last page.
	PaginationKey string `json:"pagination_key"`
}

// FundDividends returns a fund's distribution history. An empty
// paginationKey starts from the first page.
func (c *Client) FundDividends(ctx context.Context, symbol string, cat Category, paginationKey string) (*FundDividendsPage, error) {
	q := symbolParams(symbol, cat)
	q.Set("pagination_key", paginationKey)
	var out FundDividendsPage
	if err := c.get(ctx, "/market-data/fundamentals/fund-dividends/get", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FundFile is one fund document, such as a prospectus or annual report.
type FundFile struct {
	Name        string `json:"file_name"`
	URL         string `json:"url"`
	PublishDate Time   `json:"publish_date"`
	// Type is Webull's document-type code: 1 prospectus, 4 annual report,
	// 5 semi-annual report, 14 quarterly report, 17 prospectus summary.
	Type int `json:"type"`
}

// FundFiles returns a fund's published documents.
func (c *Client) FundFiles(ctx context.Context, symbol string, cat Category) ([]FundFile, error) {
	var out []FundFile
	if err := c.get(ctx, "/market-data/fundamentals/fund-files/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FundHolding is one position in a fund's portfolio.
type FundHolding struct {
	Symbol            string              `json:"target_symbol"`
	Name              string              `json:"stock_name"`
	HeldPercent       decimal.NullDecimal `json:"share_held_pct"`
	HeldChangePercent decimal.NullDecimal `json:"share_held_chg_pct"`
	MaturityDate      Time                `json:"maturity_date"`
	UpdateTime        Time                `json:"update_time"`
}

// FundHoldings returns a fund's largest positions.
func (c *Client) FundHoldings(ctx context.Context, symbol string, cat Category) ([]FundHolding, error) {
	var out []FundHolding
	if err := c.get(ctx, "/market-data/fundamentals/fund-holdings/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FundNetValue is a fund's net asset value on one date.
type FundNetValue struct {
	Date     Time            `json:"date"`
	Currency string          `json:"currency"`
	NetValue decimal.Decimal `json:"net_value"`
}

// FundNetValues returns a fund's net asset value history, newest first,
// starting from lastDate when set. Count is between 1 and 20; zero lets
// Webull apply its default of 5.
func (c *Client) FundNetValues(ctx context.Context, symbol string, cat Category, lastDate time.Time, count int) ([]FundNetValue, error) {
	q := symbolParams(symbol, cat)
	if !lastDate.IsZero() {
		q.Set("last_date", lastDate.Format("2006-01-02"))
	}
	q.SetInt("count", count)
	var out []FundNetValue
	if err := c.get(ctx, "/market-data/fundamentals/fund-net-values/get", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FundPerformance is a fund's trailing returns, as percentages.
type FundPerformance struct {
	Currency       string              `json:"currency"`
	EndDate        Time                `json:"end_date"`
	Return1M       decimal.NullDecimal `json:"return_1m"`
	Return3M       decimal.NullDecimal `json:"return_3m"`
	Return6M       decimal.NullDecimal `json:"return_6m"`
	Return1Y       decimal.NullDecimal `json:"return_1y"`
	Return3Y       decimal.NullDecimal `json:"return_3y"`
	Return5Y       decimal.NullDecimal `json:"return_5y"`
	Return10Y      decimal.NullDecimal `json:"return_10y"`
	SinceInception decimal.NullDecimal `json:"return_si"`
}

// FundPerformance returns a fund's trailing returns.
func (c *Client) FundPerformance(ctx context.Context, symbol string, cat Category) (*FundPerformance, error) {
	var out FundPerformance
	if err := c.get(ctx, "/market-data/fundamentals/fund-performances/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FundRating is one agency's rating of a fund.
type FundRating struct {
	Date   Time   `json:"rating_date"`
	Agency string `json:"rating_agency"`
	// Cycle is the period rated: "0" since establishment, or "3", "5" or
	// "10" years.
	Cycle string `json:"rating_cycle"`
	// Rating is 1 (low) through 5 (high).
	Rating int `json:"rating_results"`
}

// FundRatings returns a fund's agency ratings.
func (c *Client) FundRatings(ctx context.Context, symbol string, cat Category) ([]FundRating, error) {
	var out []FundRating
	if err := c.get(ctx, "/market-data/fundamentals/fund-ratings/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FundSplit is one share split or merge by a fund.
type FundSplit struct {
	Date Time `json:"split_date"`
	// Type is "SPLIT" or "MERGE".
	Type  string          `json:"split_type"`
	Ratio string          `json:"split_ratio"`
	From  decimal.Decimal `json:"from"`
	To    decimal.Decimal `json:"to"`
}

// FundSplits returns a fund's split history.
func (c *Client) FundSplits(ctx context.Context, symbol string, cat Category) ([]FundSplit, error) {
	var out []FundSplit
	if err := c.get(ctx, "/market-data/fundamentals/fund-splits/get", symbolParams(symbol, cat), &out); err != nil {
		return nil, err
	}
	return out, nil
}
