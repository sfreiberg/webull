package marketdata

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/sfreiberg/webull/internal/query"
)

// Categories for the other asset classes.
const (
	USOption  Category = "US_OPTION"
	USFutures Category = "US_FUTURES"
	USCrypto  Category = "US_CRYPTO"
	USEvent   Category = "US_EVENT"
)

// OptionSnapshot is the current state of an option contract, including its
// greeks and implied volatility.
type OptionSnapshot struct {
	Symbol       string `json:"symbol"`
	InstrumentID string `json:"instrument_id"`

	Price         decimal.NullDecimal `json:"price"`
	Open          decimal.NullDecimal `json:"open"`
	High          decimal.NullDecimal `json:"high"`
	Low           decimal.NullDecimal `json:"low"`
	Close         decimal.NullDecimal `json:"close"`
	PreClose      decimal.NullDecimal `json:"pre_close"`
	Volume        decimal.NullDecimal `json:"volume"`
	Change        decimal.NullDecimal `json:"change"`
	ChangeRatio   decimal.NullDecimal `json:"change_ratio"`
	LastTradeTime Millis              `json:"last_trade_time"`

	Bid       decimal.NullDecimal `json:"bid"`
	BidSize   decimal.NullDecimal `json:"bid_size"`
	Ask       decimal.NullDecimal `json:"ask"`
	AskSize   decimal.NullDecimal `json:"ask_size"`
	QuoteTime Millis              `json:"quote_time"`

	StrikePrice  decimal.NullDecimal `json:"strike_price"`
	OpenInterest decimal.NullDecimal `json:"open_interest"`
	// DealAmount is the day's traded value; returned by the API but not
	// documented.
	DealAmount decimal.NullDecimal `json:"deal_amount"`

	Delta      decimal.NullDecimal `json:"delta"`
	Gamma      decimal.NullDecimal `json:"gamma"`
	Theta      decimal.NullDecimal `json:"theta"`
	Vega       decimal.NullDecimal `json:"vega"`
	Rho        decimal.NullDecimal `json:"rho"`
	ImpliedVol decimal.NullDecimal `json:"imp_vol"`
}

// OptionSnapshots returns the current state of up to 20 option contracts,
// identified by OCC symbol.
func (c *Client) OptionSnapshots(ctx context.Context, symbols []string) ([]OptionSnapshot, error) {
	q := query.New()
	q.SetList("symbols", symbols)
	q.Set("category", string(USOption))
	var out []OptionSnapshot
	if err := c.get(ctx, "/market-data/options/snapshots/list", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OptionTick is one option trade. Side carries an undocumented "NS" value
// alongside the documented letters.
type OptionTick struct {
	Time   Millis          `json:"time"`
	Price  decimal.Decimal `json:"price"`
	Volume decimal.Decimal `json:"volume"`
	Side   TickSide        `json:"side"`
}

// OptionTicks is an option contract's recent trades, newest first.
type OptionTicks struct {
	Symbol string `json:"symbol"`
	// InstrumentID is camel-cased on the wire for this endpoint alone.
	InstrumentID string       `json:"instrumentId"`
	Ticks        []OptionTick `json:"result"`
}

// OptionTicks returns an option contract's most recent trades. count is at
// most 1200; zero lets Webull apply its default.
func (c *Client) OptionTicks(ctx context.Context, symbol string, count int) (*OptionTicks, error) {
	q := query.New()
	q.Set("symbol", symbol)
	q.Set("category", string(USOption))
	q.SetInt("count", count)
	var out OptionTicks
	if err := c.get(ctx, "/market-data/options/ticks/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AssetBarsRequest selects candles for option, futures, crypto or event
// contract symbols.
type AssetBarsRequest struct {
	// Symbols holds up to 20 symbols.
	Symbols  []string
	Timespan Timespan
	// Count is at most 1200; zero lets Webull apply its default of 200.
	Count int
	// Completed omits the bar still forming. Crypto and event contracts
	// require the flag either way and default to including it.
	Completed bool
}

func (r AssetBarsRequest) params(cat Category, flagRequired bool) query.Params {
	q := query.New()
	q.SetList("symbols", r.Symbols)
	q.Set("category", string(cat))
	q.Set("timespan", string(r.Timespan))
	q.SetInt("count", r.Count)
	if r.Completed {
		q.Set("real_time_required", "false")
	} else if flagRequired {
		q.Set("real_time_required", "true")
	}
	return q
}

// OptionBars returns candles for option contracts. Webull documents a
// {result: [...]} envelope for this endpoint; the API returns a bare array.
func (c *Client) OptionBars(ctx context.Context, req AssetBarsRequest) ([]Bars, error) {
	var out []Bars
	if err := c.get(ctx, "/market-data/options/bars/list", req.params(USOption, false), &out); err != nil {
		return nil, err
	}
	return out, nil
}
