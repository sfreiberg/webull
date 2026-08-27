package trade

import (
	"context"
	"net/url"
	"strconv"

	"github.com/shopspring/decimal"
)

// FuturesUnit is the pricing unit of a futures contract, as a numeric code.
// Webull's documentation describes this field as a string enumeration such as
// "1 - Index points"; the API transmits the integer. See FuturesUnitNames.
type FuturesUnit int

// String returns the unit's description, or its number if unknown.
func (u FuturesUnit) String() string {
	if name, ok := FuturesUnitNames[u]; ok {
		return name
	}
	return strconv.Itoa(int(u))
}

// FuturesProductClass groups futures products, such as "Equities" or "Energy".
type FuturesProductClass struct {
	ID   int    `json:"product_class_id"`
	Name string `json:"product_class_name"`
}

// FuturesProductClasses lists the classification groups for futures.
func (c *Client) FuturesProductClasses(ctx context.Context) ([]FuturesProductClass, error) {
	q := params{}
	q.set("category", string(CategoryUSFutures))
	var out []FuturesProductClass
	if err := c.get(ctx, "/trading/instruments/futures/product-classes/list", url.Values(q), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FuturesProduct is an underlying product such as "E-Mini S&P 500" (code ES).
type FuturesProduct struct {
	Name             string `json:"name"`
	Code             string `json:"code"`
	ProductClassID   int    `json:"product_class_id"`
	ProductClassName string `json:"product_class_name"`
	ExchangeCode     string `json:"exchange_code"`
}

// FuturesProducts lists futures products, optionally within one class.
// A productClassID of zero means all classes.
func (c *Client) FuturesProducts(ctx context.Context, productClassID int) ([]FuturesProduct, error) {
	q := params{}
	q.set("category", string(CategoryUSFutures))
	q.setInt("product_class_id", productClassID)
	var out []FuturesProduct
	if err := c.get(ctx, "/trading/instruments/futures/product-codes/list", url.Values(q), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// FuturesContract is a dated or continuous futures contract.
type FuturesContract struct {
	Symbol           string              `json:"symbol"`
	InstrumentID     string              `json:"instrument_id"`
	ExchangeCode     string              `json:"exchange_code"`
	Code             string              `json:"code"`
	Name             string              `json:"name"`
	ProductClassID   int                 `json:"product_class_id"`
	ProductClassName string              `json:"product_class_name"`
	Status           TradableStatus      `json:"status"`
	Currency         string              `json:"currency"`
	ContractType     FuturesContractType `json:"contract_type"`
	Settlement       Settlement          `json:"settlement"`
	Unit             FuturesUnit         `json:"unit"`

	// ContractMonth is yyyyMM. The date fields are yyyy-MM-dd.
	ContractMonth    string `json:"contract_month"`
	SettlementDate   string `json:"settlement_date"`
	FirstNoticeDate  string `json:"first_notice_date,omitempty"`
	LastNoticeDate   string `json:"last_notice_date,omitempty"`
	FirstTradingDate string `json:"first_trading_date"`
	LastTradingDate  string `json:"last_trading_date"`

	// Size is the contract multiplier; MinTick the minimum price increment.
	Size    decimal.Decimal `json:"size"`
	MinTick decimal.Decimal `json:"min_tick"`
}

// FuturesContractsRequest selects contracts by symbol or by product code.
// One of Symbols or Code must be set.
type FuturesContractsRequest struct {
	// Symbols holds contract symbols such as "ESZ5".
	Symbols []string
	// Code selects every contract for a product, such as "ES".
	Code   string
	Status TradableStatus
}

// FuturesContracts looks up futures contract definitions.
func (c *Client) FuturesContracts(ctx context.Context, req FuturesContractsRequest) ([]FuturesContract, error) {
	q := params{}
	q.set("category", string(CategoryUSFutures))
	q.setList("symbols", req.Symbols)
	q.set("code", req.Code)
	q.set("status", string(req.Status))
	var out []FuturesContract
	if err := c.get(ctx, "/trading/instruments/futures/contracts/list", url.Values(q), &out); err != nil {
		return nil, err
	}
	return out, nil
}
