package marketdata

import (
	"context"
	"strconv"

	"github.com/shopspring/decimal"

	"github.com/sfreiberg/webull/internal/query"
)

// Snapshot is the current state of a stock: last trade, top of book, the
// day's range and, when requested, extended-hours and overnight sessions.
//
// Most fields are NullDecimal because Webull omits them when there has been
// no trading in the relevant session.
type Snapshot struct {
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

	Ask       decimal.NullDecimal `json:"ask"`
	AskSize   decimal.NullDecimal `json:"ask_size"`
	Bid       decimal.NullDecimal `json:"bid"`
	BidSize   decimal.NullDecimal `json:"bid_size"`
	QuoteTime Millis              `json:"quote_time"`

	Turnover decimal.NullDecimal `json:"turnover"`
	EPS      decimal.NullDecimal `json:"eps"`
	EPSTTM   decimal.NullDecimal `json:"eps_ttm"`
	LotSize  decimal.NullDecimal `json:"lot_size"`
	BPS      decimal.NullDecimal `json:"bps"`

	// Valuation and share-count fields are returned by the API but not
	// documented.
	PERatio           decimal.NullDecimal `json:"pe_ratio"`
	PBRatio           decimal.NullDecimal `json:"pb_ratio"`
	PSRatio           decimal.NullDecimal `json:"ps_ratio"`
	Yield             decimal.NullDecimal `json:"yield"`
	MarketValue       decimal.NullDecimal `json:"market_value"`
	NegMarketValue    decimal.NullDecimal `json:"neg_market_value"`
	TotalShares       decimal.NullDecimal `json:"total_shares"`
	OutstandingShares decimal.NullDecimal `json:"out_standing_shares"`
	FiftyTwoWeekHigh  decimal.NullDecimal `json:"fifty_two_wk_high"`
	FiftyTwoWeekLow   decimal.NullDecimal `json:"fifty_two_wk_low"`
	ListStatus        string              `json:"list_status"`

	// Extended hours, populated when SnapshotsRequest.ExtendedHours is set.
	ExtendedLastPrice     decimal.NullDecimal `json:"extend_hour_last_price"`
	ExtendedHigh          decimal.NullDecimal `json:"extend_hour_high"`
	ExtendedLow           decimal.NullDecimal `json:"extend_hour_low"`
	ExtendedChange        decimal.NullDecimal `json:"extend_hour_change"`
	ExtendedChangeRatio   decimal.NullDecimal `json:"extend_hour_change_ratio"`
	ExtendedVolume        decimal.NullDecimal `json:"extend_hour_volume"`
	ExtendedLastTradeTime Millis              `json:"extend_hour_last_trade_time"`

	// Overnight session, populated when SnapshotsRequest.Overnight is set.
	OvernightPrice         decimal.NullDecimal `json:"ovn_price"`
	OvernightHigh          decimal.NullDecimal `json:"ovn_high"`
	OvernightLow           decimal.NullDecimal `json:"ovn_low"`
	OvernightVolume        decimal.NullDecimal `json:"ovn_volume"`
	OvernightChange        decimal.NullDecimal `json:"ovn_change"`
	OvernightChangeRatio   decimal.NullDecimal `json:"ovn_change_ratio"`
	OvernightLastTradeTime Millis              `json:"ovn_last_trade_time"`
	OvernightAsk           decimal.NullDecimal `json:"ovn_ask"`
	OvernightAskSize       decimal.NullDecimal `json:"ovn_ask_size"`
	OvernightBid           decimal.NullDecimal `json:"ovn_bid"`
	OvernightBidSize       decimal.NullDecimal `json:"ovn_bid_size"`
	OvernightQuoteTime     Millis              `json:"ovn_quote_time"`
}

// SnapshotsRequest selects the stocks to snapshot.
type SnapshotsRequest struct {
	// Symbols holds up to 100 symbols.
	Symbols []string
	// Category defaults to USStock.
	Category Category
	// ExtendedHours and Overnight include those sessions' fields.
	ExtendedHours bool
	Overnight     bool
}

// Snapshots returns the current state of each requested stock.
func (c *Client) Snapshots(ctx context.Context, req SnapshotsRequest) ([]Snapshot, error) {
	q := query.New()
	q.SetList("symbols", req.Symbols)
	q.Set("category", string(category(req.Category)))
	if req.ExtendedHours {
		q.Set("extend_hour_required", "true")
	}
	if req.Overnight {
		q.Set("overnight_required", "true")
	}
	var out []Snapshot
	if err := c.get(ctx, "/market-data/stocks/snapshots/list", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Depth is the order book for one stock.
type Depth struct {
	Symbol       string  `json:"symbol"`
	InstrumentID string  `json:"instrument_id"`
	QuoteTime    Millis  `json:"quote_time"`
	Asks         []Level `json:"asks"`
	Bids         []Level `json:"bids"`
}

// Level is one price level of the book. Order and Broker breakdowns are
// present only at depths the key is entitled to.
type Level struct {
	Price  decimal.Decimal `json:"price"`
	Size   decimal.Decimal `json:"size"`
	Orders []LevelOrder    `json:"order,omitempty"`
	Broker []LevelBroker   `json:"broker,omitempty"`
}

// LevelOrder is one market participant's size at a level.
type LevelOrder struct {
	MPID string          `json:"mpid"`
	Size decimal.Decimal `json:"size"`
}

// LevelBroker identifies a broker at a level.
type LevelBroker struct {
	ID   string `json:"bid"`
	Name string `json:"name"`
}

// DepthRequest selects a stock's order book.
type DepthRequest struct {
	Symbol   string
	Category Category
	// Levels is how many price levels to return. Webull rejects a number
	// beyond the key's entitlement: a key with Level 1 data may ask for one.
	// Zero lets Webull apply its default.
	Levels    int
	Overnight bool
}

// Depth returns a stock's order book.
func (c *Client) Depth(ctx context.Context, req DepthRequest) (*Depth, error) {
	q := query.New()
	q.Set("symbol", req.Symbol)
	q.Set("category", string(category(req.Category)))
	q.SetInt("depth", req.Levels)
	q.Set("overnight_required", strconv.FormatBool(req.Overnight))
	var out Depth
	if err := c.get(ctx, "/market-data/stocks/depths/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Tick is one trade.
type Tick struct {
	Time    Millis          `json:"time"`
	Price   decimal.Decimal `json:"price"`
	Volume  decimal.Decimal `json:"volume"`
	Side    TickSide        `json:"side"`
	Session TradingSession  `json:"trading_session"`
}

// Ticks is a stock's recent trades, newest first.
type Ticks struct {
	Symbol       string `json:"symbol"`
	InstrumentID string `json:"instrument_id"`
	Ticks        []Tick `json:"result"`
}

// TicksRequest selects a stock's recent trades.
type TicksRequest struct {
	Symbol   string
	Category Category
	// Count is between 1 and 1000; zero lets Webull apply its default of 30.
	Count int
	// Sessions restricts results to the given sessions. Empty means all.
	Sessions []TradingSession
}

// Ticks returns a stock's most recent trades.
func (c *Client) Ticks(ctx context.Context, req TicksRequest) (*Ticks, error) {
	q := query.New()
	q.Set("symbol", req.Symbol)
	q.Set("category", string(category(req.Category)))
	q.SetInt("count", req.Count)
	q.SetList("trading_sessions", sessions(req.Sessions))
	var out Ticks
	if err := c.get(ctx, "/market-data/stocks/ticks/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Bar is one OHLCV candle.
type Bar struct {
	Time    Time            `json:"time"`
	Open    decimal.Decimal `json:"open"`
	High    decimal.Decimal `json:"high"`
	Low     decimal.Decimal `json:"low"`
	Close   decimal.Decimal `json:"close"`
	Volume  decimal.Decimal `json:"volume"`
	Session TradingSession  `json:"trading_session"`
}

// Bars is one stock's candles.
type Bars struct {
	Symbol       string `json:"symbol"`
	InstrumentID string `json:"instrument_id"`
	Bars         []Bar  `json:"result"`
}

// BarsRequest selects candles for one or more stocks.
type BarsRequest struct {
	// Symbols holds up to 100 symbols.
	Symbols  []string
	Category Category
	Timespan Timespan
	// Count is at most 1200 (1650 for Minute); zero lets Webull apply its
	// default of 200.
	Count int
	// Completed omits the bar still forming.
	Completed bool
	Sessions  []TradingSession
	// Start and End bound the range; zero values are omitted.
	Start Millis
	End   Millis
}

type barsBody struct {
	Symbols          []string `json:"symbols"`
	Category         Category `json:"category"`
	Timespan         Timespan `json:"timespan"`
	Count            int      `json:"count,omitzero"`
	RealTimeRequired *bool    `json:"real_time_required,omitempty"`
	TradingSessions  string   `json:"trading_sessions,omitzero"`
	StartTime        int64    `json:"start_time,omitzero"`
	EndTime          int64    `json:"end_time,omitzero"`
}

// Bars returns candles for each requested stock.
func (c *Client) Bars(ctx context.Context, req BarsRequest) ([]Bars, error) {
	body := barsBody{
		Symbols:  req.Symbols,
		Category: category(req.Category),
		Timespan: req.Timespan,
		Count:    req.Count,
	}
	if req.Completed {
		f := false
		body.RealTimeRequired = &f
	}
	if s := sessions(req.Sessions); len(s) > 0 {
		body.TradingSessions = joinSessions(s)
	}
	if !req.Start.IsZero() {
		body.StartTime = req.Start.UnixMilli()
	}
	if !req.End.IsZero() {
		body.EndTime = req.End.UnixMilli()
	}
	var out struct {
		Result []Bars `json:"result"`
	}
	if err := c.post(ctx, "/market-data/stocks/bars/list", body, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

// Footprint is one interval's traded volume broken down by price and side.
type Footprint struct {
	Time      Time            `json:"time"`
	Session   TradingSession  `json:"trading_session"`
	Total     decimal.Decimal `json:"total"`
	Delta     decimal.Decimal `json:"delta"`
	BuyTotal  decimal.Decimal `json:"buy_total"`
	SellTotal decimal.Decimal `json:"sell_total"`
	// BuyDetail and SellDetail map price to volume.
	BuyDetail  map[string]decimal.Decimal `json:"buy_detail"`
	SellDetail map[string]decimal.Decimal `json:"sell_detail"`
}

// Footprints is one stock's footprint chart.
type Footprints struct {
	Symbol       string      `json:"symbol"`
	InstrumentID string      `json:"instrument_id"`
	Footprints   []Footprint `json:"result"`
}

// FootprintsRequest selects footprint data. Requires the FOOTPRINT
// subscription.
type FootprintsRequest struct {
	Symbols  []string
	Category Category
	// Timespan is one of Second5, Second15, Minute, Minute5 or Minute30.
	Timespan Timespan
	// Count is at most 1200; zero lets Webull apply its default of 200.
	Count int
	// Completed omits the interval still forming.
	Completed bool
	// Session selects one session; Overnight is not supported.
	Session TradingSession
}

// Footprints returns footprint charts for each requested stock.
func (c *Client) Footprints(ctx context.Context, req FootprintsRequest) ([]Footprints, error) {
	q := query.New()
	q.SetList("symbols", req.Symbols)
	q.Set("category", string(category(req.Category)))
	q.Set("timespan", string(req.Timespan))
	q.SetInt("count", req.Count)
	q.Set("real_time_required", strconv.FormatBool(!req.Completed))
	q.Set("trading_sessions", string(req.Session))
	var out []Footprints
	if err := c.get(ctx, "/market-data/stocks/footprints/list", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Imbalance is one auction imbalance reading.
type Imbalance struct {
	Symbol       string          `json:"symbol"`
	InstrumentID string          `json:"instrument_id"`
	Time         Millis          `json:"imbalance_time"`
	Type         ImbalanceType   `json:"imbalance_action_type"`
	RefPrice     decimal.Decimal `json:"imbalance_ref_price"`
	NearPrice    decimal.Decimal `json:"imbalance_near_price"`
	FarPrice     decimal.Decimal `json:"imbalance_far_price"`
}

// ImbalanceSnapshot is the current auction imbalance for a stock.
type ImbalanceSnapshot struct {
	Imbalance
	PairedShares    decimal.Decimal `json:"paired_shares"`
	ImbalanceShares decimal.Decimal `json:"imbalance_shares"`
	Side            string          `json:"imbalance_side"`
	VarIndicator    string          `json:"imbalance_var_indicator"`
}

// ImbalanceRequest selects a stock's opening or closing auction imbalance.
// Requires the STOCK QUOTES LV2 subscription.
type ImbalanceRequest struct {
	Symbol   string
	Category Category
	Type     ImbalanceType
}

// ImbalanceBars returns the history of auction imbalance readings.
func (c *Client) ImbalanceBars(ctx context.Context, req ImbalanceRequest) ([]Imbalance, error) {
	var out []Imbalance
	if err := c.get(ctx, "/market-data/stocks/noii-bars/list", imbalanceParams(req), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ImbalanceSnapshot returns the current auction imbalance.
func (c *Client) ImbalanceSnapshot(ctx context.Context, req ImbalanceRequest) (*ImbalanceSnapshot, error) {
	var out ImbalanceSnapshot
	if err := c.get(ctx, "/market-data/stocks/noii-snapshots/list", imbalanceParams(req), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func imbalanceParams(req ImbalanceRequest) query.Params {
	q := query.New()
	q.Set("symbol", req.Symbol)
	q.Set("category", string(category(req.Category)))
	q.Set("imbalance_action_type", string(req.Type))
	return q
}

func sessions(s []TradingSession) []string {
	out := make([]string, 0, len(s))
	for _, v := range s {
		out = append(out, string(v))
	}
	return out
}

func joinSessions(s []string) string {
	q := query.New()
	q.SetList("s", s)
	return q.Values().Get("s")
}
