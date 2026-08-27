package trade

import (
	"context"

	"github.com/shopspring/decimal"
)

// EventCategory groups event contract series, such as Economics or Sports.
type EventCategory struct {
	// ID is numeric on the wire although Webull documents it as a string.
	ID   int               `json:"category_id"`
	Code EventCategoryCode `json:"category_code"`
	Name string            `json:"category_name"`
}

// EventCategories lists all event contract categories.
func (c *Client) EventCategories(ctx context.Context) ([]EventCategory, error) {
	var out []EventCategory
	if err := c.get(ctx, "/trading/instruments/event-contracts/categories/list", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// EventSeries is a template for recurring events, such as "Fed Meeting".
type EventSeries struct {
	SeriesID  string            `json:"series_id"`
	Symbol    string            `json:"symbol"`
	Name      string            `json:"name"`
	Category  EventCategoryCode `json:"category"`
	Frequency EventFrequency    `json:"frequency"`
}

// EventSeriesRequest filters a series lookup.
type EventSeriesRequest struct {
	Category EventCategoryCode
	// Symbols holds up to 100 series symbols.
	Symbols       []string
	PaginationKey string
}

// EventSeriesPage is one page of event series.
type EventSeriesPage struct {
	Series        []EventSeries `json:"data"`
	PaginationKey string        `json:"pagination_key"`
}

// EventSeries lists event contract series.
func (c *Client) EventSeries(ctx context.Context, req EventSeriesRequest) (*EventSeriesPage, error) {
	q := params{}
	q.set("category", string(req.Category))
	q.setList("symbols", req.Symbols)
	q.set("pagination_key", req.PaginationKey)
	var out EventSeriesPage
	if err := c.get(ctx, "/trading/instruments/event-contracts/series/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Event is one occurrence within a series, such as a particular Fed meeting.
type Event struct {
	SeriesID  string      `json:"series_id"`
	Symbol    string      `json:"symbol"`
	Name      string      `json:"name"`
	ShortName string      `json:"short_name"`
	Status    EventStatus `json:"status"`
	// StrikeDate is yyyy-MM-dd; StrikePeriod is free text such as "Q4 2026".
	StrikeDate        string `json:"strike_date"`
	StrikePeriod      string `json:"strike_period"`
	MutuallyExclusive bool   `json:"mutually_exclusive"`
}

// EventsRequest selects events within a series.
type EventsRequest struct {
	SeriesSymbol string
	// Symbols holds up to 100 event symbols.
	Symbols []string
	Status  EventStatus
}

// Events lists the events in a series. Webull does not paginate this endpoint.
func (c *Client) Events(ctx context.Context, req EventsRequest) ([]Event, error) {
	q := params{}
	q.set("series_symbol", req.SeriesSymbol)
	q.setList("symbols", req.Symbols)
	q.set("status", string(req.Status))
	var out []Event
	if err := c.get(ctx, "/trading/instruments/event-contracts/events/list", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// EventMarket is a tradable yes/no contract on an event.
type EventMarket struct {
	SeriesID     string `json:"series_id"`
	SeriesSymbol string `json:"series_symbol"`
	SeriesName   string `json:"series_name"`
	EventSymbol  string `json:"event_symbol"`
	EventName    string `json:"event_name"`
	InstrumentID string `json:"instrument_id"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
	YesCondition string `json:"yes_condition"`

	Status         EventMarketStatus `json:"status"`
	TradableStatus TradableStatus    `json:"tradable_status"`
	CanCloseEarly  bool              `json:"can_close_early"`
	Fractionable   bool              `json:"fractionable"`

	// Dates are yyyy-MM-dd.
	LastTradingDate string `json:"last_trading_date"`
	ExpectedExpDate string `json:"expected_exp_date"`
	LatestExpDate   string `json:"latest_exp_date"`
	PayoutDate      string `json:"payout_date"`

	PriceRanges []PriceRange `json:"price_ranges"`
}

// PriceRange is a band of permitted prices and the tick within it.
type PriceRange struct {
	Start decimal.Decimal `json:"start"`
	End   decimal.Decimal `json:"end"`
	Step  decimal.Decimal `json:"step"`
}

// EventMarketsRequest filters an event market lookup.
type EventMarketsRequest struct {
	SeriesSymbol string
	EventSymbol  string
	// Symbols holds up to 100 market symbols.
	Symbols []string
	// ExpirationDateAfter is yyyy-MM-dd.
	ExpirationDateAfter string
	PaginationKey       string
}

// EventMarketsPage is one page of event markets.
type EventMarketsPage struct {
	Markets       []EventMarket `json:"data"`
	PaginationKey string        `json:"pagination_key"`
}

// EventMarkets lists tradable event contract markets.
func (c *Client) EventMarkets(ctx context.Context, req EventMarketsRequest) (*EventMarketsPage, error) {
	q := params{}
	q.set("series_symbol", req.SeriesSymbol)
	q.set("event_symbol", req.EventSymbol)
	q.setList("symbols", req.Symbols)
	q.set("expiration_date_after", req.ExpirationDateAfter)
	q.set("pagination_key", req.PaginationKey)
	var out EventMarketsPage
	if err := c.get(ctx, "/trading/instruments/event-contracts/markets/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
