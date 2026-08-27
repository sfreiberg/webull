package trade

import (
	"context"

	"github.com/sfreiberg/webull/internal/query"

	"github.com/shopspring/decimal"
)

// StockProfile is reference data for an equity instrument.
type StockProfile struct {
	Name         string           `json:"name"`
	InstrumentID string           `json:"instrument_id"`
	ExchangeCode string           `json:"exchange_code"`
	Category     Category         `json:"category"`
	Symbol       string           `json:"symbol"`
	Status       TradableStatus   `json:"status"`
	SubCategory  StockSubCategory `json:"sub_category"`
	Currency     string           `json:"currency"`

	Shortable                 bool `json:"shortable"`
	Fractionable              bool `json:"fractionable"`
	Marginable                bool `json:"marginable"`
	OvernightTradingSupported bool `json:"overnight_trading_supported"`
	EasyToBorrow              bool `json:"easy_to_borrow"`
	CryptoETF                 bool `json:"crypto_etf"`
	// SingleStockETF and InverseETF are returned by the API but not documented.
	SingleStockETF bool `json:"single_stock_etf"`
	InverseETF     bool `json:"inverse_etf"`

	MarginRequirementLong  decimal.NullDecimal `json:"margin_requirement_long"`
	MarginRequirementShort decimal.NullDecimal `json:"margin_requirement_short"`
	IntradayMarginLong     decimal.NullDecimal `json:"intraday_margin_long"`
	IntradayMarginShort    decimal.NullDecimal `json:"intraday_margin_short"`
	MaintenanceMarginLong  decimal.NullDecimal `json:"maintenance_margin_long"`
	MaintenanceMarginShort decimal.NullDecimal `json:"maintenance_margin_short"`
	LotSize                decimal.NullDecimal `json:"lot_size"`

	// ETFLeveragedFlag is "YES" or "NO"; ETFLeveragedFactor is the multiple.
	ETFLeveragedFlag   string              `json:"etf_leveraged_flag"`
	ETFLeveragedFactor decimal.NullDecimal `json:"etf_leveraged_factor"`
}

// StockProfilesRequest filters an equity lookup. Either Symbols or a
// SubCategory browse is expected; SubCategory is ignored when Symbols is set.
type StockProfilesRequest struct {
	// Symbols holds up to 100 symbols.
	Symbols     []string
	Status      TradableStatus
	SubCategory StockSubCategory
	// PaginationKey continues a previous page. Leave empty for the first.
	PaginationKey string
}

// StockProfilesPage is one page of stock profiles.
type StockProfilesPage struct {
	Profiles []StockProfile `json:"data"`
	// PaginationKey fetches the next page, and is empty on the last one.
	PaginationKey string `json:"pagination_key"`
}

// StockProfiles looks up equity reference data.
func (c *Client) StockProfiles(ctx context.Context, req StockProfilesRequest) (*StockProfilesPage, error) {
	q := query.New()
	q.Set("category", string(CategoryUSStock))
	q.SetList("symbols", req.Symbols)
	q.Set("status", string(req.Status))
	q.Set("sub_category", string(req.SubCategory))
	q.Set("pagination_key", req.PaginationKey)

	var out StockProfilesPage
	if err := c.get(ctx, "/trading/instruments/stocks/profiles/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CryptoProfile is reference data for a crypto trading pair.
type CryptoProfile struct {
	Name         string         `json:"name"`
	InstrumentID string         `json:"instrument_id"`
	ExchangeCode string         `json:"exchange_code"`
	Category     Category       `json:"category"`
	Symbol       string         `json:"symbol"`
	Status       TradableStatus `json:"status"`
	Currency     string         `json:"currency"`

	MinTradeAmount   decimal.NullDecimal `json:"min_trade_amt"`
	MaxTradeAmount   decimal.NullDecimal `json:"max_trade_amt"`
	MinTradeQuantity decimal.NullDecimal `json:"min_trade_qty"`
	MaxTradeQuantity decimal.NullDecimal `json:"max_trade_qty"`
	PriceStep        decimal.NullDecimal `json:"price_step"`
	LotSize          decimal.NullDecimal `json:"lot_size"`
}

// CryptoProfilesRequest filters a crypto lookup.
type CryptoProfilesRequest struct {
	// Symbols holds up to 100 pairs such as "BTCUSD".
	Symbols       []string
	Status        TradableStatus
	PaginationKey string
}

// CryptoProfilesPage is one page of crypto profiles.
type CryptoProfilesPage struct {
	Profiles      []CryptoProfile `json:"data"`
	PaginationKey string          `json:"pagination_key"`
}

// CryptoProfiles looks up crypto reference data.
func (c *Client) CryptoProfiles(ctx context.Context, req CryptoProfilesRequest) (*CryptoProfilesPage, error) {
	q := query.New()
	q.Set("category", string(CategoryUSCrypto))
	q.SetList("symbols", req.Symbols)
	q.Set("status", string(req.Status))
	q.Set("pagination_key", req.PaginationKey)

	var out CryptoProfilesPage
	if err := c.get(ctx, "/trading/instruments/crypto/profiles/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OptionContract is the static definition of a listed option.
type OptionContract struct {
	InstrumentID   string         `json:"instrument_id"`
	Symbol         string         `json:"symbol"`
	Status         ListingStatus  `json:"status"`
	TradableStatus TradableStatus `json:"tradable_status"`
	// ExpirationDate is in yyyy-MM-dd form, after any corporate-action
	// adjustment. InitExpirationDate is the original date and is returned by
	// the API but not documented.
	ExpirationDate     string `json:"expiration_date"`
	InitExpirationDate string `json:"init_expiration_date"`

	RootSymbol             string `json:"root_symbol"`
	UnderlyingSymbol       string `json:"underlying_symbol"`
	UnderlyingInstrumentID string `json:"underlying_instrument_id"`
	UnderlyingType         string `json:"underlying_type"`

	OptionType       OptionType      `json:"option_type"`
	Style            OptionStyle     `json:"style"`
	StrikePrice      decimal.Decimal `json:"strike_price"`
	Multiplier       decimal.Decimal `json:"multiplier"`
	SettlementMethod string          `json:"settlement_method"`
	ExpiredCycle     string          `json:"expired_cycle"`
	// PPInd is the Penny Program indicator.
	PPInd           bool     `json:"ppind"`
	Currency        string   `json:"currency"`
	DefType         string   `json:"def_type"`
	ListedExchanges []string `json:"listed_exchanges"`

	// Deliverables is populated only when requested with ShowDeliverables.
	Deliverables []OptionDeliverable `json:"deliverables"`
}

// OptionDeliverable is what one contract delivers on exercise. After a
// corporate action a single contract may deliver several things.
type OptionDeliverable struct {
	AssetType            string          `json:"asset_type"`
	Symbol               string          `json:"symbol"`
	InstrumentID         string          `json:"instrument_id"`
	Amount               decimal.Decimal `json:"amount"`
	AllocationPercentage decimal.Decimal `json:"allocation_percentage"`
	SettlementType       string          `json:"settlement_type"`
	SettlementMethod     string          `json:"settlement_method"`
	SettlementStatus     string          `json:"settlement_status"`
}

// OptionContractsRequest filters an option chain lookup.
type OptionContractsRequest struct {
	// OptionSymbols selects specific OCC symbols.
	OptionSymbols []string
	// UnderlyingSymbols selects chains by underlying.
	UnderlyingSymbols []string
	// Status defaults to Listing when empty.
	Status ListingStatus
	// StartDate and EndDate are yyyy-MM-dd expiration bounds.
	StartDate string
	EndDate   string
	// RootSymbol filters by series, such as "SPXW".
	RootSymbol string
	OptionType OptionType
	Style      OptionStyle
	// StrikePriceGTE and StrikePriceLTE bound the strike, inclusive.
	StrikePriceGTE *decimal.Decimal
	StrikePriceLTE *decimal.Decimal
	// PPInd filters on the Penny Program indicator; nil means either.
	PPInd *bool
	// ShowDeliverables includes the Deliverables array in each contract.
	ShowDeliverables bool
	PaginationKey    string
}

// OptionContractsPage is one page of option contracts.
type OptionContractsPage struct {
	Contracts     []OptionContract `json:"data"`
	PaginationKey string           `json:"pagination_key"`
}

// OptionContracts looks up option contracts.
func (c *Client) OptionContracts(ctx context.Context, req OptionContractsRequest) (*OptionContractsPage, error) {
	q := query.New()
	q.Set("category", string(CategoryUSOption))
	q.SetList("option_symbols", req.OptionSymbols)
	q.SetList("underlying_symbols", req.UnderlyingSymbols)
	q.Set("status", string(req.Status))
	q.Set("start_date", req.StartDate)
	q.Set("end_date", req.EndDate)
	q.Set("root_symbol", req.RootSymbol)
	q.Set("option_type", string(req.OptionType))
	q.Set("style", string(req.Style))
	if req.StrikePriceGTE != nil {
		q.Set("strike_price_gte", req.StrikePriceGTE.String())
	}
	if req.StrikePriceLTE != nil {
		q.Set("strike_price_lte", req.StrikePriceLTE.String())
	}
	q.SetBool("ppind", req.PPInd)
	if req.ShowDeliverables {
		q.Set("show_deliverables", "TRUE")
	}
	q.Set("pagination_key", req.PaginationKey)

	var out OptionContractsPage
	if err := c.get(ctx, "/trading/instruments/options/contracts/list", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
