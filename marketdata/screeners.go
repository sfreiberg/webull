package marketdata

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/sfreiberg/webull/internal/query"
)

// screenerList decodes a screener response in either of the shapes Webull
// uses: the bare array the sandbox returns, or the {"data": [...]} envelope
// its formatter documentation describes. The same tolerance the bars
// endpoints needed, in the other direction.
type screenerList[T any] []T

func (l *screenerList[T]) UnmarshalJSON(b []byte) error {
	if strings.HasPrefix(strings.TrimSpace(string(b)), "[") {
		var bare []T
		if err := json.Unmarshal(b, &bare); err != nil {
			return err
		}
		*l = bare
		return nil
	}
	var wrapped struct {
		Data []T `json:"data"`
	}
	if err := json.Unmarshal(b, &wrapped); err != nil {
		return err
	}
	*l = wrapped.Data
	return nil
}

// RankPeriod is the time window a gainers-losers ranking is computed over.
type RankPeriod string

// Ranking periods.
const (
	RankPreMarket   RankPeriod = "PRE_MARKET"
	RankAfterMarket RankPeriod = "AFTER_MARKET"
	Rank3Min        RankPeriod = "MIN_3"
	Rank5Min        RankPeriod = "MIN_5"
	Rank1Day        RankPeriod = "DAY_1"
	Rank5Day        RankPeriod = "DAY_5"
	Rank1Month      RankPeriod = "MONTH_1"
	Rank3Month      RankPeriod = "MONTH_3"
	Rank52Week      RankPeriod = "WEEK_52"
)

// ScreenerSort is the field a screener ranking is ordered by.
type ScreenerSort string

// Screener sort fields. Yield and Dividend apply to the high-dividend
// screener only.
const (
	SortChangeRatio    ScreenerSort = "CHANGE_RATIO"
	SortRelativeVolume ScreenerSort = "RELATIVE_VOLUME_10D"
	SortMarketValue    ScreenerSort = "MARKET_VALUE"
	SortClose          ScreenerSort = "CLOSE"
	SortPrice          ScreenerSort = "PRICE"
	SortPETTM          ScreenerSort = "PE_TTM"
	SortHigh           ScreenerSort = "HIGH"
	SortLow            ScreenerSort = "LOW"
	SortAmplitude      ScreenerSort = "AMPLITUDE"
	SortTurnover       ScreenerSort = "TURNOVER"
	SortVolume         ScreenerSort = "VOLUME"
	SortYield          ScreenerSort = "YIELD"
	SortDividend       ScreenerSort = "DIVIDEND"
	// SortChangeRatio52W is the 52-week screener's server default: the
	// change since the 52-week extreme rather than the daily change.
	SortChangeRatio52W ScreenerSort = "CHANGE_RATIO_52W"
)

// SortDirection orders a ranking.
type SortDirection string

// Sort directions.
const (
	Ascending  SortDirection = "ASC"
	Descending SortDirection = "DESC"
)

// ActivityMetric is the measure the top-active screener ranks by.
type ActivityMetric string

// Activity metrics.
const (
	ByVolume         ActivityMetric = "VOLUME"
	ByRelativeVolume ActivityMetric = "RELATIVE_VOLUME_10D"
	ByTurnover       ActivityMetric = "TURNOVER"
	ByTurnoverRate   ActivityMetric = "TURNOVER_RATE"
	ByAmplitude      ActivityMetric = "AMPLITUDE"
)

// Week52Rank selects which 52-week extreme the screener reports.
type Week52Rank string

// 52-week rank types.
const (
	NewHigh  Week52Rank = "NEW_HIGH"
	NearHigh Week52Rank = "NEAR_HIGH"
	NewLow   Week52Rank = "NEW_LOW"
	NearLow  Week52Rank = "NEAR_LOW"
)

// SectorPeriod is the window sector statistics are computed over.
type SectorPeriod string

// Sector statistics periods.
const (
	Sector1Day   SectorPeriod = "D1"
	Sector5Day   SectorPeriod = "D5"
	Sector1Month SectorPeriod = "MO1"
	Sector3Month SectorPeriod = "MO3"
)

// SectorAggregate is the statistic sectors are ranked by.
type SectorAggregate string

// Sector aggregates.
const (
	SectorMarketValue SectorAggregate = "MARKET_VALUE"
	SectorVolume      SectorAggregate = "VOLUME"
)

// ScreenerStock is one row of the gainers-losers or top-active screener.
//
// Turnover is null for US-market keys without the entitlement Webull calls
// "Nb authorization"; Currency and PETTM are returned but not documented.
type ScreenerStock struct {
	InstrumentID   string      `json:"instrument_id"`
	Symbol         string      `json:"symbol"`
	Name           string      `json:"name"`
	ExchangeCode   string      `json:"exchange_code"`
	CurrencyCode   string      `json:"currency_code"`
	Currency       string      `json:"currency"`
	PreClose       NullDecimal `json:"pre_close"`
	Open           NullDecimal `json:"open"`
	High           NullDecimal `json:"high"`
	Low            NullDecimal `json:"low"`
	Close          NullDecimal `json:"close"`
	Price          NullDecimal `json:"price"`
	Change         NullDecimal `json:"change"`
	ChangeRatio    NullDecimal `json:"change_ratio"`
	Volume         NullDecimal `json:"volume"`
	Turnover       NullDecimal `json:"turnover"`
	TurnoverRate   NullDecimal `json:"turnover_rate"`
	MarketValue    NullDecimal `json:"market_value"`
	Amplitude      NullDecimal `json:"amplitude"`
	RelativeVolume NullDecimal `json:"relative_volume_10d"`
	PETTM          NullDecimal `json:"pe_ttm"`
}

// GainersLosersRequest selects a price-change ranking.
type GainersLosersRequest struct {
	// Period is the change window; the server default is Rank1Day.
	Period RankPeriod
	// Category defaults to USStock.
	Category Category
	// SortBy defaults to SortChangeRatio on the server.
	SortBy ScreenerSort
	// Direction defaults to Descending, which ranks gainers; Ascending
	// ranks losers.
	Direction SortDirection
}

// GainersLosers returns stocks ranked by price change.
func (c *Client) GainersLosers(ctx context.Context, req GainersLosersRequest) ([]ScreenerStock, error) {
	q := query.New()
	q.Set("rank_type", string(req.Period))
	q.Set("category", string(category(req.Category)))
	q.Set("sort_by", string(req.SortBy))
	q.Set("direction", string(req.Direction))
	var out screenerList[ScreenerStock]
	if err := c.get(ctx, "/market-data/screeners/gainers-losers/list", q, &out); err != nil {
		return nil, err
	}
	return []ScreenerStock(out), nil
}

// TopActiveRequest selects an activity ranking.
type TopActiveRequest struct {
	// Category defaults to USStock.
	Category Category
	// Metric defaults to ByVolume on the server.
	Metric ActivityMetric
	// SortBy defaults to SortVolume on the server.
	SortBy    ScreenerSort
	Direction SortDirection
}

// TopActive returns the most actively traded stocks.
func (c *Client) TopActive(ctx context.Context, req TopActiveRequest) ([]ScreenerStock, error) {
	q := query.New()
	q.Set("category", string(category(req.Category)))
	q.Set("rank_type", string(req.Metric))
	q.Set("sort_by", string(req.SortBy))
	q.Set("direction", string(req.Direction))
	var out screenerList[ScreenerStock]
	if err := c.get(ctx, "/market-data/screeners/top-actives/list", q, &out); err != nil {
		return nil, err
	}
	return []ScreenerStock(out), nil
}

// DividendStock is one row of the high-dividend screener.
type DividendStock struct {
	InstrumentID string      `json:"instrument_id"`
	Category     Category    `json:"category"`
	Symbol       string      `json:"symbol"`
	Name         string      `json:"name"`
	ExchangeCode string      `json:"exchange_code"`
	Currency     string      `json:"currency"`
	Close        NullDecimal `json:"close"`
	Change       NullDecimal `json:"change"`
	ChangeRatio  NullDecimal `json:"change_ratio"`
	Price        NullDecimal `json:"price"`
	Volume       NullDecimal `json:"volume"`
	MarketValue  NullDecimal `json:"market_value"`
	TurnoverRate NullDecimal `json:"turnover_rate"`
	Amplitude    NullDecimal `json:"amplitude"`
	High         NullDecimal `json:"high"`
	Low          NullDecimal `json:"low"`
	Turnover     NullDecimal `json:"turnover"`
	Yield        NullDecimal `json:"yield"`
	Dividend     NullDecimal `json:"dividend"`
	ExDate       Time        `json:"ex_date"`
	PETTM        NullDecimal `json:"pe_ttm"`
}

// HighDividendRequest selects a dividend-yield ranking.
type HighDividendRequest struct {
	// Category defaults to USStock.
	Category Category
	// SortBy defaults to SortYield on the server.
	SortBy ScreenerSort
	// Direction defaults to Descending on the server.
	Direction SortDirection
}

// HighDividend returns stocks ranked by dividend yield.
func (c *Client) HighDividend(ctx context.Context, req HighDividendRequest) ([]DividendStock, error) {
	q := query.New()
	q.Set("category", string(category(req.Category)))
	q.Set("sort_by", string(req.SortBy))
	q.Set("direction", string(req.Direction))
	var out screenerList[DividendStock]
	if err := c.get(ctx, "/market-data/screeners/high-dividend-ranks/list", q, &out); err != nil {
		return nil, err
	}
	return []DividendStock(out), nil
}

// Week52Stock is one row of the 52-week high-low screener.
type Week52Stock struct {
	InstrumentID string      `json:"instrument_id"`
	Category     Category    `json:"category"`
	Symbol       string      `json:"symbol"`
	Name         string      `json:"name"`
	ExchangeCode string      `json:"exchange_code"`
	Currency     string      `json:"currency"`
	Close        NullDecimal `json:"close"`
	Change       NullDecimal `json:"change"`
	ChangeRatio  NullDecimal `json:"change_ratio"`
	Price        NullDecimal `json:"price"`
	Volume       NullDecimal `json:"volume"`
	MarketValue  NullDecimal `json:"market_value"`
	TurnoverRate NullDecimal `json:"turnover_rate"`
	Amplitude    NullDecimal `json:"amplitude"`
	High         NullDecimal `json:"high"`
	Low          NullDecimal `json:"low"`
	Turnover     NullDecimal `json:"turnover"`
	// Week1Price and Week52Price are the extreme this week and over 52
	// weeks; ChangeRatio52W is the change since the 52-week extreme.
	Week1Price    NullDecimal `json:"price_1w"`
	Week52Price   NullDecimal `json:"price_52w"`
	ChangeRatio52 NullDecimal `json:"change_ratio_52w"`
	PETTM         NullDecimal `json:"pe_ttm"`
}

// Week52Request selects a 52-week high-low ranking.
type Week52Request struct {
	// Category defaults to USStock.
	Category Category
	// Rank selects the extreme reported; the server default is NewHigh.
	Rank Week52Rank
	// SortBy defaults to SortChangeRatio52W on the server.
	SortBy ScreenerSort
	// Direction defaults to Ascending on the server.
	Direction SortDirection
}

// Week52HighLow returns stocks at or near their 52-week high or low.
func (c *Client) Week52HighLow(ctx context.Context, req Week52Request) ([]Week52Stock, error) {
	q := query.New()
	q.Set("category", string(category(req.Category)))
	q.Set("rank_type", string(req.Rank))
	q.Set("sort_by", string(req.SortBy))
	q.Set("direction", string(req.Direction))
	var out screenerList[Week52Stock]
	if err := c.get(ctx, "/market-data/screeners/week52-high-low/list", q, &out); err != nil {
		return nil, err
	}
	return []Week52Stock(out), nil
}

// SectorLeader is a leading stock within a sector.
type SectorLeader struct {
	InstrumentID string `json:"instrument_id"`
	Symbol       string `json:"symbol"`
	Name         string `json:"name"`
}

// Sector is one market sector's aggregate statistics.
type Sector struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	ChangeRatio NullDecimal    `json:"change_ratio"`
	Volume      NullDecimal    `json:"volume"`
	MarketValue NullDecimal    `json:"market_value"`
	Declined    NullDecimal    `json:"declined"`
	Advanced    NullDecimal    `json:"advanced"`
	Flat        NullDecimal    `json:"flat"`
	Leaders     []SectorLeader `json:"data"`
}

// SectorsPage is one page of market sectors.
type SectorsPage struct {
	Sectors []Sector `json:"data"`
	// PaginationKey continues the listing; empty on the last page.
	PaginationKey string `json:"pagination_key"`
}

// UnmarshalJSON accepts the {"data": [...], "pagination_key": ...} envelope
// the sandbox returns, or the bare array Webull's formatter documentation
// describes, which cannot paginate.
func (p *SectorsPage) UnmarshalJSON(b []byte) error {
	if strings.HasPrefix(strings.TrimSpace(string(b)), "[") {
		var bare []Sector
		if err := json.Unmarshal(b, &bare); err != nil {
			return err
		}
		*p = SectorsPage{Sectors: bare}
		return nil
	}
	type plain SectorsPage
	var decoded plain
	if err := json.Unmarshal(b, &decoded); err != nil {
		return err
	}
	*p = SectorsPage(decoded)
	return nil
}

// MarketSectorsRequest selects a sector ranking.
type MarketSectorsRequest struct {
	// Category defaults to USStock.
	Category Category
	// Aggregate defaults to SectorMarketValue on the server.
	Aggregate SectorAggregate
	// Period defaults to Sector1Day on the server.
	Period    SectorPeriod
	Direction SortDirection
	// PaginationKey continues a previous page. Leave empty for the first.
	PaginationKey string
}

// MarketSectors returns market sectors ranked by the chosen aggregate.
func (c *Client) MarketSectors(ctx context.Context, req MarketSectorsRequest) (*SectorsPage, error) {
	q := query.New()
	q.Set("category", string(category(req.Category)))
	q.Set("agg_type", string(req.Aggregate))
	q.Set("period", string(req.Period))
	q.Set("direction", string(req.Direction))
	q.Set("pagination_key", req.PaginationKey)
	var out SectorsPage
	if err := c.get(ctx, "/market-data/screeners/market-sectors/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SectorStock is one stock within a sector detail listing.
type SectorStock struct {
	InstrumentID string      `json:"instrument_id"`
	Category     Category    `json:"category"`
	Symbol       string      `json:"symbol"`
	Name         string      `json:"name"`
	ExchangeCode string      `json:"exchange_code"`
	Currency     string      `json:"currency"`
	Close        NullDecimal `json:"close"`
	ChangeRatio  NullDecimal `json:"change_ratio"`
	Price        NullDecimal `json:"price"`
	Volume       NullDecimal `json:"volume"`
	MarketValue  NullDecimal `json:"market_value"`
	TurnoverRate NullDecimal `json:"turnover_rate"`
	Amplitude    NullDecimal `json:"amplitude"`
	High         NullDecimal `json:"high"`
	Low          NullDecimal `json:"low"`
}

// SectorDetail is one sector's statistics with its constituent stocks.
type SectorDetail struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	ChangeRatio NullDecimal   `json:"change_ratio"`
	Declined    NullDecimal   `json:"declined"`
	Advanced    NullDecimal   `json:"advanced"`
	Flat        NullDecimal   `json:"flat"`
	Stocks      []SectorStock `json:"data"`
	// PaginationKey continues the stock listing; empty on the last page.
	PaginationKey string `json:"pagination_key"`
}

// SectorDetailRequest selects one sector's constituents.
type SectorDetailRequest struct {
	// SectorID comes from MarketSectors.
	SectorID string
	// Category defaults to USStock.
	Category Category
	// Period defaults to Sector1Day on the server.
	Period SectorPeriod
	// SortBy defaults to SortChangeRatio on the server.
	SortBy    ScreenerSort
	Direction SortDirection
	// PaginationKey continues a previous page. Leave empty for the first.
	PaginationKey string
}

// SectorDetail returns one sector's statistics and constituent stocks.
func (c *Client) SectorDetail(ctx context.Context, req SectorDetailRequest) (*SectorDetail, error) {
	q := query.New()
	q.Set("sector_id", req.SectorID)
	q.Set("category", string(category(req.Category)))
	q.Set("period", string(req.Period))
	q.Set("sort_by", string(req.SortBy))
	q.Set("direction", string(req.Direction))
	q.Set("pagination_key", req.PaginationKey)
	var out SectorDetail
	if err := c.get(ctx, "/market-data/screeners/market-sectors/get", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
