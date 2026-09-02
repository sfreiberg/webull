package marketdata

import (
	"context"
	"strconv"

	"github.com/sfreiberg/webull/internal/query"
)

// OptionSnapshot is the current state of an option contract, including its
// greeks and implied volatility.
type OptionSnapshot struct {
	Symbol       string `json:"symbol"`
	InstrumentID string `json:"instrument_id"`

	Price         NullDecimal `json:"price"`
	Open          NullDecimal `json:"open"`
	High          NullDecimal `json:"high"`
	Low           NullDecimal `json:"low"`
	Close         NullDecimal `json:"close"`
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

	StrikePrice  NullDecimal `json:"strike_price"`
	OpenInterest NullDecimal `json:"open_interest"`
	// DealAmount is the day's traded value; returned by the API but not
	// documented.
	DealAmount NullDecimal `json:"deal_amount"`

	Delta      NullDecimal `json:"delta"`
	Gamma      NullDecimal `json:"gamma"`
	Theta      NullDecimal `json:"theta"`
	Vega       NullDecimal `json:"vega"`
	Rho        NullDecimal `json:"rho"`
	ImpliedVol NullDecimal `json:"imp_vol"`
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

// AssetTicks is an option or futures contract's recent trades, newest first.
// Option ticks carry an undocumented "NS" Side alongside the documented
// letters, and no Session.
type AssetTicks struct {
	Symbol string `json:"symbol"`
	// InstrumentID is camel-cased on the wire for the option and futures tick
	// endpoints, unlike every other endpoint in the API.
	InstrumentID string `json:"instrumentId"`
	Ticks        []Tick `json:"result"`
}

// OptionTicks returns an option contract's most recent trades. count is at
// most 1200; zero lets Webull apply its default.
func (c *Client) OptionTicks(ctx context.Context, symbol string, count int) (*AssetTicks, error) {
	q := symbolParams(symbol, USOption)
	q.SetInt("count", count)
	var out AssetTicks
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
	// Completed omits the bar still forming.
	//
	// How this reaches the wire depends on the asset class. Crypto and event
	// contracts require the real_time_required parameter and receive it
	// either way; options accept it and receive it only when Completed is
	// set; futures do not document it and never receive it.
	Completed bool
}

func (r AssetBarsRequest) params(cat Category) query.Params {
	q := query.New()
	q.SetList("symbols", r.Symbols)
	q.Set("category", string(cat))
	q.Set("timespan", string(r.Timespan))
	q.SetInt("count", r.Count)
	switch cat {
	case USCrypto, USEvent:
		q.Set("real_time_required", strconv.FormatBool(!r.Completed))
	case USOption:
		if r.Completed {
			q.Set("real_time_required", "false")
		}
	}
	return q
}

// bars fetches candles from path, accepting either response shape.
func (c *Client) bars(ctx context.Context, path string, req AssetBarsRequest, cat Category) ([]Bars, error) {
	var out barsList
	if err := c.get(ctx, path, req.params(cat), &out); err != nil {
		return nil, err
	}
	return []Bars(out), nil
}

// OptionBars returns candles for option contracts.
func (c *Client) OptionBars(ctx context.Context, req AssetBarsRequest) ([]Bars, error) {
	return c.bars(ctx, "/market-data/options/bars/list", req, USOption)
}
