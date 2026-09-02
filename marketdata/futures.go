package marketdata

import (
	"context"

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

	Price         NullDecimal `json:"price"`
	Open          NullDecimal `json:"open"`
	High          NullDecimal `json:"high"`
	Low           NullDecimal `json:"low"`
	PreClose      NullDecimal `json:"pre_close"`
	Volume        NullDecimal `json:"volume"`
	Change        NullDecimal `json:"change"`
	ChangeRatio   NullDecimal `json:"change_ratio"`
	LastTradeTime Millis      `json:"last_trade_time"`

	Bid       NullDecimal `json:"bid"`
	BidSize   NullDecimal `json:"bid_size"`
	Ask       NullDecimal `json:"ask"`
	AskSize   NullDecimal `json:"ask_size"`
	QuoteTime Millis      `json:"quote_time"`

	OpenInterest NullDecimal `json:"open_interest"`
	SettlePrice  NullDecimal `json:"settle_price"`
	SettleDate   Time        `json:"settle_date"`
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

// FuturesTicks returns a futures contract's most recent trades. count is at
// most 1200; zero lets Webull apply its default.
func (c *Client) FuturesTicks(ctx context.Context, symbol string, count int) (*AssetTicks, error) {
	q := symbolParams(symbol, USFutures)
	q.SetInt("count", count)
	var out AssetTicks
	if err := c.get(ctx, "/market-data/futures/ticks/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FuturesDepth returns a futures contract's order book. Requires the
// FUTURES LV2 subscription; levels is how many price levels per side.
func (c *Client) FuturesDepth(ctx context.Context, symbol string, levels int) (*Depth, error) {
	q := symbolParams(symbol, USFutures)
	q.SetInt("depth", levels)
	var out Depth
	if err := c.get(ctx, "/market-data/futures/depths/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// FuturesBars returns candles for futures contracts.
func (c *Client) FuturesBars(ctx context.Context, req AssetBarsRequest) ([]Bars, error) {
	return c.bars(ctx, "/market-data/futures/bars/list", req, USFutures)
}

// FuturesFootprints returns footprint charts for futures contracts. Requires
// the FOOTPRINT subscription. req.Category is ignored: the futures endpoint
// takes only USFutures.
func (c *Client) FuturesFootprints(ctx context.Context, req FootprintsRequest) ([]Footprints, error) {
	var out []Footprints
	if err := c.get(ctx, "/market-data/futures/footprints/list", footprintParams(req, USFutures), &out); err != nil {
		return nil, err
	}
	return out, nil
}
