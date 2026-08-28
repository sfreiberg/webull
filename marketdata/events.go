package marketdata

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/sfreiberg/webull/internal/query"
)

// EventSnapshot is the current state of an event contract market: a binary
// contract with a yes side and a no side.
type EventSnapshot struct {
	Symbol       string `json:"symbol"`
	InstrumentID string `json:"instrument_id"`
	Name         string `json:"name"`

	Price         decimal.NullDecimal `json:"price"`
	Volume        decimal.NullDecimal `json:"volume"`
	OpenInterest  decimal.NullDecimal `json:"open_interest"`
	LastTradeTime Millis              `json:"last_trade_time"`

	YesBid     decimal.NullDecimal `json:"yes_bid"`
	YesBidSize decimal.NullDecimal `json:"yes_bid_size"`
	YesAsk     decimal.NullDecimal `json:"yes_ask"`
	YesAskSize decimal.NullDecimal `json:"yes_ask_size"`
	NoBid      decimal.NullDecimal `json:"no_bid"`
	NoBidSize  decimal.NullDecimal `json:"no_bid_size"`
	NoAsk      decimal.NullDecimal `json:"no_ask"`
	NoAskSize  decimal.NullDecimal `json:"no_ask_size"`
}

// EventSnapshots returns the current state of event contract markets.
func (c *Client) EventSnapshots(ctx context.Context, symbols []string) ([]EventSnapshot, error) {
	q := query.New()
	q.SetList("symbols", symbols)
	q.Set("category", string(USEvent))
	var out []EventSnapshot
	if err := c.get(ctx, "/market-data/event-contracts/snapshots/list", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// EventDepth is the order book of an event contract market, one book per
// outcome. Sides with no orders are absent or empty.
type EventDepth struct {
	Symbol       string  `json:"symbol"`
	InstrumentID string  `json:"instrument_id"`
	QuoteTime    Millis  `json:"quote_time"`
	YesBids      []Level `json:"yes_bids"`
	YesAsks      []Level `json:"yes_asks"`
	NoBids       []Level `json:"no_bids"`
	NoAsks       []Level `json:"no_asks"`
}

// EventDepth returns an event contract market's order book. levels is per
// side; zero lets Webull apply its default of ten.
func (c *Client) EventDepth(ctx context.Context, symbol string, levels int) (*EventDepth, error) {
	q := query.New()
	q.Set("symbol", symbol)
	q.Set("category", string(USEvent))
	q.SetInt("depth", levels)
	var out EventDepth
	if err := c.get(ctx, "/market-data/event-contracts/depths/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EventOutcome is the side of an event contract trade.
type EventOutcome string

// Event outcomes.
const (
	Yes EventOutcome = "yes"
	No  EventOutcome = "no"
)

// EventTick is one event contract trade, reported with both outcomes' prices.
type EventTick struct {
	Time     Millis          `json:"time"`
	TradeID  string          `json:"trade_id"`
	Side     EventOutcome    `json:"side"`
	YesPrice decimal.Decimal `json:"yes_price"`
	NoPrice  decimal.Decimal `json:"no_price"`
	Volume   decimal.Decimal `json:"volume"`
}

// EventTicks is an event contract market's recent trades, newest first.
type EventTicks struct {
	Symbol       string      `json:"symbol"`
	InstrumentID string      `json:"instrument_id"`
	Ticks        []EventTick `json:"result"`
}

// EventTicks returns an event contract market's most recent trades. count is
// at most 1200; zero lets Webull apply its default of 30.
func (c *Client) EventTicks(ctx context.Context, symbol string, count int) (*EventTicks, error) {
	q := query.New()
	q.Set("symbol", symbol)
	q.Set("category", string(USEvent))
	q.SetInt("count", count)
	var out EventTicks
	if err := c.get(ctx, "/market-data/event-contracts/ticks/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EventBars returns candles for event contract markets.
func (c *Client) EventBars(ctx context.Context, req AssetBarsRequest) ([]Bars, error) {
	var out []Bars
	if err := c.get(ctx, "/market-data/event-contracts/bars/list", req.params(USEvent, true), &out); err != nil {
		return nil, err
	}
	return out, nil
}
