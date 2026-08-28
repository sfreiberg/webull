package marketdata

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/sfreiberg/webull/internal/query"
)

// FuturesSnapshot is the current state of a futures contract.
//
// The sandbox serves futures market data for a single symbol, "MESmain", the
// continuous Micro E-mini S&P 500; every other symbol is rejected with
// "Only these symbols are supported". A snapshot for a continuous symbol
// reports the resolved contract, such as "MESU6", in Symbol.
type FuturesSnapshot struct {
	Symbol       string `json:"symbol"`
	InstrumentID string `json:"instrument_id"`

	Price         decimal.NullDecimal `json:"price"`
	Open          decimal.NullDecimal `json:"open"`
	High          decimal.NullDecimal `json:"high"`
	Low           decimal.NullDecimal `json:"low"`
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

	OpenInterest decimal.NullDecimal `json:"open_interest"`
	SettlePrice  decimal.NullDecimal `json:"settle_price"`
	SettleDate   Time                `json:"settle_date"`
}

// FuturesSnapshots returns the current state of up to 20 futures contracts.
func (c *Client) FuturesSnapshots(ctx context.Context, symbols []string) ([]FuturesSnapshot, error) {
	q := query.New()
	q.SetList("symbols", symbols)
	q.Set("category", string(USFutures))
	var out []FuturesSnapshot
	if err := c.get(ctx, "/market-data/futures/snapshots/list", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FuturesTicks is a futures contract's recent trades, newest first.
type FuturesTicks struct {
	Symbol string `json:"symbol"`
	// InstrumentID is camel-cased on the wire for this endpoint.
	InstrumentID string `json:"instrumentId"`
	Ticks        []Tick `json:"result"`
}

// FuturesTicks returns a futures contract's most recent trades. count is at
// most 1200; zero lets Webull apply its default.
func (c *Client) FuturesTicks(ctx context.Context, symbol string, count int) (*FuturesTicks, error) {
	q := query.New()
	q.Set("symbol", symbol)
	q.Set("category", string(USFutures))
	q.SetInt("count", count)
	var out FuturesTicks
	if err := c.get(ctx, "/market-data/futures/ticks/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FuturesDepth returns a futures contract's order book. Requires the
// FUTURES LV2 subscription; levels is how many price levels per side.
func (c *Client) FuturesDepth(ctx context.Context, symbol string, levels int) (*Depth, error) {
	q := query.New()
	q.Set("symbol", symbol)
	q.Set("category", string(USFutures))
	q.SetInt("depth", levels)
	var out Depth
	if err := c.get(ctx, "/market-data/futures/depths/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FuturesBars returns candles for futures contracts. As with options, the
// API returns a bare array rather than the documented envelope.
func (c *Client) FuturesBars(ctx context.Context, req AssetBarsRequest) ([]Bars, error) {
	var out []Bars
	if err := c.get(ctx, "/market-data/futures/bars/list", req.params(USFutures, false), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FuturesFootprints returns footprint charts for futures contracts. Requires
// the FOOTPRINT subscription.
func (c *Client) FuturesFootprints(ctx context.Context, req FootprintsRequest) ([]Footprints, error) {
	req.Category = USFutures
	var out []Footprints
	if err := c.get(ctx, "/market-data/futures/footprints/list", footprintParams(req), &out); err != nil {
		return nil, err
	}
	return out, nil
}
