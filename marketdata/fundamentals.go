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

func symbolParams(symbol string, cat Category) query.Params {
	q := query.New()
	q.Set("symbol", symbol)
	q.Set("category", string(category(cat)))
	return q
}
