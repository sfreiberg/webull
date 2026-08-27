package trade

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Side is the direction of an order.
type Side string

// Order sides. Short is a short sale of equities.
const (
	Buy   Side = "BUY"
	Sell  Side = "SELL"
	Short Side = "SHORT"
)

// OrderType determines how an order executes.
//
// Not every type is available to every asset class: event contracts accept
// Limit only, crypto accepts Market, Limit and StopLossLimit, and options do
// not accept TrailingStopLoss. Batch placement accepts Market and Limit. An
// order of a type its instrument does not permit fails locally.
type OrderType string

// Order types.
const (
	Market           OrderType = "MARKET"
	Limit            OrderType = "LIMIT"
	StopLoss         OrderType = "STOP_LOSS"
	StopLossLimit    OrderType = "STOP_LOSS_LIMIT"
	TrailingStopLoss OrderType = "TRAILING_STOP_LOSS"
	MarketOnOpen     OrderType = "MARKET_ON_OPEN"
	MarketOnClose    OrderType = "MARKET_ON_CLOSE"
	LimitOnOpen      OrderType = "LIMIT_ON_OPEN"
)

// TimeInForce is how long an order remains active.
//
// Equities accept every value; options and futures accept Day and GTC;
// crypto accepts Day, GTC and IOC. Webull enforces regular trading hours for
// Day orders, rejecting one placed after the close, so anything that must
// run at any hour should use GTC.
type TimeInForce string

// Time-in-force values. GTD requires Order.ExpireDate.
const (
	Day TimeInForce = "DAY"
	GTC TimeInForce = "GTC"
	IOC TimeInForce = "IOC"
	GTD TimeInForce = "GTD"
	FOK TimeInForce = "FOK"
)

// OrderStatus is the lifecycle state of an order.
type OrderStatus string

// Order statuses.
const (
	StatusPending       OrderStatus = "PENDING"
	StatusSubmitted     OrderStatus = "SUBMITTED"
	StatusCancelled     OrderStatus = "CANCELLED"
	StatusFilled        OrderStatus = "FILLED"
	StatusFailed        OrderStatus = "FAILED"
	StatusPartialFilled OrderStatus = "PARTIAL_FILLED"
)

// ComboType is an order's role within a group. Normal is a standalone order;
// every other value only makes sense inside a Combo.
type ComboType string

// Combo roles. See Bracket, OTO, OCO and OTOCO for the groups they form.
const (
	Normal ComboType = "NORMAL"
	// RoleMaster is the primary order of a bracket, OTO or OTOCO group.
	RoleMaster ComboType = "MASTER"
	// RoleStopProfit and RoleStopLoss are the take-profit and stop-loss
	// orders of a bracket.
	RoleStopProfit ComboType = "STOP_PROFIT"
	RoleStopLoss   ComboType = "STOP_LOSS"
	// RoleOTO, RoleOCO and RoleOTOCO mark the dependent orders of those
	// groups.
	RoleOTO   ComboType = "OTO"
	RoleOCO   ComboType = "OCO"
	RoleOTOCO ComboType = "OTOCO"
)

// EntrustType is whether quantity is expressed in units or in cash. ByAmount
// is accepted for equities (fractional shares) and crypto only.
type EntrustType string

// Entrust types.
const (
	ByQuantity EntrustType = "QTY"
	ByAmount   EntrustType = "AMOUNT"
)

// TradingSession restricts when a US equity order may execute.
type TradingSession string

// Trading sessions.
const (
	SessionAll   TradingSession = "ALL"
	SessionCore  TradingSession = "CORE"
	SessionNight TradingSession = "NIGHT"
)

// PositionIntent is the effect of an option order on the position.
type PositionIntent string

// Position intents.
const (
	BuyToOpen   PositionIntent = "BUY_TO_OPEN"
	BuyToClose  PositionIntent = "BUY_TO_CLOSE"
	SellToOpen  PositionIntent = "SELL_TO_OPEN"
	SellToClose PositionIntent = "SELL_TO_CLOSE"
)

// TrailingType is how a trailing stop's offset is expressed. Both kinds are
// accepted for equities and futures, on either side and with GTC as well as
// Day. Options do not accept trailing stops at all. The wire values are
// case-sensitive.
type TrailingType string

// Trailing types.
const (
	TrailByAmount     TrailingType = "AMOUNT"
	TrailByPercentage TrailingType = "PERCENTAGE"
)

// AlgoType selects an algorithmic execution strategy.
type AlgoType string

// Algorithm types.
const (
	TWAP AlgoType = "TWAP"
	VWAP AlgoType = "VWAP"
	POV  AlgoType = "POV"
)

// EventTradeMode is how an event contract order's size is expressed.
type EventTradeMode string

// Event trade modes.
const (
	TradeInAmount   EventTradeMode = "TRADE_IN_AMOUNT"
	TradeInContract EventTradeMode = "TRADE_IN_CONTRACT"
)

// LegDirection is whether an option order legs into or out of a position.
type LegDirection string

// Leg directions.
const (
	LegIn  LegDirection = "LEG_IN"
	LegOut LegDirection = "LEG_OUT"
)

// LegInStrategy is the multi-leg strategy a position becomes after legging in.
type LegInStrategy string

// Leg-in strategies.
const (
	Vertical   LegInStrategy = "VERTICAL"
	Calendar   LegInStrategy = "CALENDAR"
	Strangle   LegInStrategy = "STRANGLE"
	Straddle   LegInStrategy = "STRADDLE"
	IronCondor LegInStrategy = "IRON_CONDOR"
	Butterfly  LegInStrategy = "BUTTERFLY"
	Custom     LegInStrategy = "CUSTOM"
)

// Price returns a set NullDecimal from a literal, for optional price fields.
// It panics on a malformed string, like regexp.MustCompile; use
// decimal.NewFromString for untrusted input.
func Price(s string) decimal.NullDecimal {
	return decimal.NewNullDecimal(decimal.RequireFromString(s))
}

// Order describes an order to preview or place.
//
// Only Symbol, Side, Type, Quantity and (for limit and stop-limit orders)
// LimitPrice are needed for a plain equity order; the SDK supplies Webull's
// required boilerplate. Optional decimal fields are decimal.NullDecimal and are
// omitted from the request when unset, so a zero price remains expressible.
//
// Set InstrumentType for futures, crypto and event contracts. Each asset
// class permits different sides, order types, times in force and quantity
// precision; those rules are checked before any request is sent, and an
// event contract order additionally needs EventOutcome.
type Order struct {
	// ClientOrderID identifies the order to Webull and is the key for cancel,
	// replace and lookup. Webull requires 10 to 40 characters, unique per
	// account. If empty, PlaceOrder and PreviewOrder generate one and write it
	// back here before sending, so the caller holds it even if no response
	// arrives.
	//
	// An ID is consumed once Webull accepts the order, so an Order value
	// describes one placement: placing it a second time returns
	// ErrDuplicateOrder. A rejected placement does not consume the ID, so a
	// corrected Order may be resent as is.
	ClientOrderID string `json:"client_order_id"`

	// InstrumentType defaults to InstrumentEquity, or to InstrumentOption when
	// Legs is set.
	InstrumentType InstrumentType `json:"instrument_type,omitzero"`
	// ComboType defaults to Normal.
	ComboType ComboType `json:"combo_type,omitzero"`
	// EntrustType defaults to ByQuantity.
	EntrustType EntrustType `json:"entrust_type,omitzero"`
	// Market defaults to "US".
	Market string `json:"market,omitzero"`
	// TradingSession applies to US equities and defaults to SessionCore; Webull
	// rejects equity orders without one even though its documentation marks
	// the field optional.
	TradingSession TradingSession `json:"support_trading_session,omitzero"`

	Symbol      string      `json:"symbol"`
	Side        Side        `json:"side"`
	Type        OrderType   `json:"order_type"`
	TimeInForce TimeInForce `json:"time_in_force,omitzero"`
	// ExpireDate is yyyy-MM-dd and is required when TimeInForce is GTD.
	ExpireDate string `json:"expire_date,omitzero"`

	Quantity decimal.NullDecimal `json:"quantity,omitzero"`
	// TotalCashAmount places a fractional equity order by dollar value; set
	// EntrustType to ByAmount.
	TotalCashAmount decimal.NullDecimal `json:"total_cash_amount,omitzero"`
	LimitPrice      decimal.NullDecimal `json:"limit_price,omitzero"`
	StopPrice       decimal.NullDecimal `json:"stop_price,omitzero"`

	TrailingType     TrailingType        `json:"trailing_type,omitzero"`
	TrailingStopStep decimal.NullDecimal `json:"trailing_stop_step,omitzero"`

	// CurrentAsk and CurrentBid are accepted by preview only, for stock and
	// option orders.
	CurrentAsk decimal.NullDecimal `json:"current_ask,omitzero"`
	CurrentBid decimal.NullDecimal `json:"current_bid,omitzero"`

	AlgoType         AlgoType            `json:"algo_type,omitzero"`
	TargetVolPercent decimal.NullDecimal `json:"target_vol_percent,omitzero"`
	MaxTargetPercent decimal.NullDecimal `json:"max_target_percent,omitzero"`
	// AlgoStartTime and AlgoEndTime are HH:mm:ss in US Eastern time.
	AlgoStartTime string `json:"algo_start_time,omitzero"`
	AlgoEndTime   string `json:"algo_end_time,omitzero"`

	EventOutcome   EventOutcome   `json:"event_outcome,omitzero"`
	EventTradeMode EventTradeMode `json:"event_trade_mode,omitzero"`

	// Option fields. OptionStrategy defaults to StrategySingle when there is
	// exactly one leg.
	OptionStrategy OptionStrategy `json:"option_strategy,omitzero"`
	PositionIntent PositionIntent `json:"position_intent,omitzero"`
	LegDirection   LegDirection   `json:"leg_in_or_out,omitzero"`
	PositionID     string         `json:"position_id,omitzero"`
	LegInStrategy  LegInStrategy  `json:"leg_in_strategy,omitzero"`
	Legs           []OrderLeg     `json:"legs,omitempty"`
}

// OrderLeg is one leg of an option order. LegFromSymbol builds one from an
// OCC symbol. Covered and collar strategies also take a stock leg, with
// InstrumentType set to InstrumentEquity, no option fields, and Quantity in
// shares — it is never defaulted from the order's contract count.
type OrderLeg struct {
	Side     Side                `json:"side"`
	Quantity decimal.NullDecimal `json:"quantity,omitzero"`
	// Market defaults to "US"; InstrumentType to InstrumentOption.
	Market         string         `json:"market,omitzero"`
	InstrumentType InstrumentType `json:"instrument_type,omitzero"`
	// Symbol is the option's root symbol, such as "AAPL" or "SPXW". For index
	// weeklies this differs from the underlying ("SPX"), and Webull rejects the
	// underlying; LegFromSymbol sets it correctly.
	Symbol      string              `json:"symbol"`
	StrikePrice decimal.NullDecimal `json:"strike_price,omitzero"`
	// ExpireDate is yyyy-MM-dd.
	ExpireDate string     `json:"option_expire_date,omitzero"`
	OptionType OptionType `json:"option_type,omitzero"`
}

// occSymbol matches an OCC option symbol as Webull renders it: root, yymmdd,
// C or P, and the strike times 1000 as eight digits.
var occSymbol = regexp.MustCompile(`^([A-Z][A-Z0-9.]*)(\d{6})([CP])(\d{8})$`)

// LegFromSymbol builds a leg from an OCC option symbol such as
// "AAPL261218C00240000", the form returned by OptionContracts. The symbol's
// root becomes the leg's Symbol, which is what Webull expects. Side and
// Quantity are left for the caller.
func LegFromSymbol(symbol string) (OrderLeg, error) {
	m := occSymbol.FindStringSubmatch(symbol)
	if m == nil {
		return OrderLeg{}, fmt.Errorf("trade: %q is not an OCC option symbol", symbol)
	}
	expiry, err := time.Parse("060102", m[2])
	if err != nil {
		return OrderLeg{}, fmt.Errorf("trade: %q has an invalid expiry: %w", symbol, err)
	}
	optType := Call
	if m[3] == "P" {
		optType = Put
	}
	// The strike is eight digits in thousandths; the pattern guarantees it
	// parses.
	strike := decimal.RequireFromString(m[4]).Shift(-3)
	return OrderLeg{
		Symbol:      m[1],
		ExpireDate:  expiry.Format("2006-01-02"),
		OptionType:  optType,
		StrikePrice: decimal.NewNullDecimal(strike),
	}, nil
}

// ErrInvalidOrder wraps local validation failures, which are detected before
// any request is sent.
var ErrInvalidOrder = errors.New("trade: invalid order")

// ErrDuplicateOrder is returned when Webull rejects a placement because the
// ClientOrderID was already accepted for another order. The underlying
// *webull.APIError remains reachable with errors.As.
var ErrDuplicateOrder = errors.New("trade: client order id already used")

// duplicateOrderCode is Webull's code for a reused client order ID.
const duplicateOrderCode = "OPENAPI_TRADE_PLACE_ORDER_REPEAT"

// classify wraps errors carrying codes this package gives a name to.
func classify(err error) error {
	var coded interface{ ErrorCode() string }
	if errors.As(err, &coded) && coded.ErrorCode() == duplicateOrderCode {
		return fmt.Errorf("%w: %w", ErrDuplicateOrder, err)
	}
	return err
}

// clear resets an unset NullDecimal to its zero value so that omitzero omits
// it. A NullDecimal that has been assigned and then unmarshalled from null
// keeps its old payload with Valid false, which would otherwise serialise as
// an explicit null.
func clear(d *decimal.NullDecimal) {
	if !d.Valid {
		*d = decimal.NullDecimal{}
	}
}

const (
	clientOrderIDMin = 10
	clientOrderIDMax = 40
)

// newClientOrderID returns a 32-character hex identifier.
func newClientOrderID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("trade: cannot read random bytes for a client order id: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// prepare fills defaults and validates. It mutates o so that generated values
// are visible to the caller.
func (o *Order) prepare() error {
	if o.ClientOrderID == "" {
		o.ClientOrderID = newClientOrderID()
	}
	if n := len(o.ClientOrderID); n < clientOrderIDMin || n > clientOrderIDMax {
		return fmt.Errorf("%w: ClientOrderID must be %d to %d characters, got %d",
			ErrInvalidOrder, clientOrderIDMin, clientOrderIDMax, n)
	}
	if o.InstrumentType == "" {
		o.InstrumentType = InstrumentEquity
		if len(o.Legs) > 0 {
			o.InstrumentType = InstrumentOption
		}
	}
	if o.ComboType == "" {
		o.ComboType = Normal
	}
	if o.EntrustType == "" {
		o.EntrustType = ByQuantity
		if o.TotalCashAmount.Valid && !o.Quantity.Valid {
			o.EntrustType = ByAmount
		}
	}
	if o.Market == "" {
		o.Market = "US"
	}
	if o.TradingSession == "" && o.InstrumentType == InstrumentEquity {
		o.TradingSession = SessionCore
	}
	if o.TimeInForce == "" {
		o.TimeInForce = Day
	}
	for _, d := range []*decimal.NullDecimal{&o.Quantity, &o.TotalCashAmount, &o.LimitPrice, &o.StopPrice,
		&o.TrailingStopStep, &o.CurrentAsk, &o.CurrentBid, &o.TargetVolPercent, &o.MaxTargetPercent} {
		clear(d)
	}
	if len(o.Legs) == 0 {
		o.Legs = nil
	}

	var problems []string
	if o.Symbol == "" {
		problems = append(problems, "Symbol is required")
	}
	if o.Side == "" {
		problems = append(problems, "Side is required")
	}
	if o.Type == "" {
		problems = append(problems, "Type is required")
	}
	switch o.Type {
	case Limit, LimitOnOpen:
		if !o.LimitPrice.Valid {
			problems = append(problems, "LimitPrice is required for "+string(o.Type))
		}
	case StopLoss:
		if !o.StopPrice.Valid {
			problems = append(problems, "StopPrice is required for STOP_LOSS")
		}
	case StopLossLimit:
		if !o.LimitPrice.Valid || !o.StopPrice.Valid {
			problems = append(problems, "LimitPrice and StopPrice are required for STOP_LOSS_LIMIT")
		}
	case TrailingStopLoss:
		if o.TrailingType == "" || !o.TrailingStopStep.Valid {
			problems = append(problems, "TrailingType and TrailingStopStep are required for TRAILING_STOP_LOSS")
		}
	}
	if o.TimeInForce == GTD && o.ExpireDate == "" {
		problems = append(problems, "ExpireDate is required for GTD")
	}
	if o.EntrustType == ByQuantity && !o.Quantity.Valid {
		problems = append(problems, "Quantity is required")
	}
	if o.EntrustType == ByAmount && !o.TotalCashAmount.Valid {
		problems = append(problems, "TotalCashAmount is required for AMOUNT orders")
	}
	problems = o.validateAsset(problems)

	if o.InstrumentType == InstrumentOption {
		if len(o.Legs) == 0 {
			problems = append(problems, "option orders need at least one leg")
		}
		if o.OptionStrategy == "" && len(o.Legs) == 1 {
			o.OptionStrategy = StrategySingle
		}
		if o.OptionStrategy == "" && len(o.Legs) > 1 {
			problems = append(problems, "multi-leg option orders need an OptionStrategy")
		}
		if o.OptionStrategy == StrategySingle && len(o.Legs) != 1 {
			problems = append(problems, "SINGLE takes exactly one leg")
		}
		if o.OptionStrategy != "" && o.OptionStrategy != StrategySingle {
			// Webull does not validate leg counts at preview time, so a
			// malformed strategy would surface only on placement.
			if len(o.Legs) < 2 {
				problems = append(problems, string(o.OptionStrategy)+" needs at least two legs")
			}
			if o.ComboType != Normal {
				problems = append(problems, "multi-leg option orders support only the NORMAL combo type")
			}
		}
		for i := range o.Legs {
			leg := &o.Legs[i]
			if leg.Market == "" {
				leg.Market = "US"
			}
			if leg.InstrumentType == "" {
				leg.InstrumentType = InstrumentOption
			}
			if leg.Side == "" {
				leg.Side = o.Side
			}
			clear(&leg.StrikePrice)
			switch leg.InstrumentType {
			case InstrumentOption:
				// An option leg's quantity is in contracts, like the order's.
				if !leg.Quantity.Valid {
					leg.Quantity = o.Quantity
				}
				if leg.Symbol == "" || !leg.StrikePrice.Valid || leg.ExpireDate == "" || leg.OptionType == "" {
					problems = append(problems, fmt.Sprintf("leg %d needs Symbol, StrikePrice, ExpireDate and OptionType", i))
				}
			case InstrumentEquity:
				// A stock leg of a covered or collar strategy is sized in
				// shares, so the order's contract count is never a sensible
				// default; the caller must set it.
				if leg.Symbol == "" {
					problems = append(problems, fmt.Sprintf("leg %d needs Symbol", i))
				}
				if !leg.Quantity.Valid {
					problems = append(problems, fmt.Sprintf("leg %d: a stock leg needs its Quantity in shares", i))
				}
			default:
				problems = append(problems, fmt.Sprintf("leg %d: legs must be OPTION or EQUITY", i))
			}
		}
	}

	if len(problems) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidOrder, strings.Join(problems, "; "))
	}
	return nil
}

// OrderReceipt identifies an order that was placed, replaced or cancelled.
type OrderReceipt struct {
	ClientOrderID string `json:"client_order_id"`
	// OrderID is Webull's identifier. Cancel, replace and lookup are keyed by
	// ClientOrderID, not by this.
	OrderID            string `json:"order_id"`
	ClientComboOrderID string `json:"client_combo_order_id"`
	ComboOrderID       string `json:"combo_order_id"`
}

// OrderPreview estimates the cost of an order without placing it.
type OrderPreview struct {
	EstimatedCost           decimal.Decimal `json:"estimated_cost"`
	EstimatedTransactionFee decimal.Decimal `json:"estimated_transaction_fee"`
	// Currency is returned by the API but not documented.
	Currency string `json:"currency"`
}

type orderEnvelope struct {
	AccountID string  `json:"account_id"`
	NewOrders []Order `json:"new_orders"`
}

// forPlacement returns a copy of o with preview-only fields removed, so that
// previewing and then placing the same Order value does not forward them.
func (o *Order) forPlacement() Order {
	c := *o
	c.CurrentAsk, c.CurrentBid = decimal.NullDecimal{}, decimal.NullDecimal{}
	return c
}

// PreviewOrder estimates cost and fees for an order without placing it. It
// applies the same defaults and validation as PlaceOrder.
func (c *Client) PreviewOrder(ctx context.Context, accountID string, order *Order) (*OrderPreview, error) {
	if err := order.prepare(); err != nil {
		return nil, err
	}
	var out OrderPreview
	if err := c.post(ctx, "/trading/orders/preview", orderEnvelope{AccountID: accountID, NewOrders: []Order{*order}}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PlaceOrder submits an order.
//
// If order.ClientOrderID is empty a value is generated and written back before
// the request is sent. Should the call fail with a transport error, the
// outcome is unknown: the order may have been accepted. Use Order with that
// ClientOrderID to find out before retrying — PlaceOrder itself never retries.
func (c *Client) PlaceOrder(ctx context.Context, accountID string, order *Order) (*OrderReceipt, error) {
	if err := order.prepare(); err != nil {
		return nil, err
	}
	var out OrderReceipt
	if err := c.post(ctx, "/trading/orders/place", orderEnvelope{AccountID: accountID, NewOrders: []Order{order.forPlacement()}}, &out); err != nil {
		return nil, classify(err)
	}
	return &out, nil
}

// OrderModification changes a working order, identified by ClientOrderID.
// Unset fields are left as they are.
type OrderModification struct {
	ClientOrderID string              `json:"client_order_id"`
	Type          OrderType           `json:"order_type,omitzero"`
	TimeInForce   TimeInForce         `json:"time_in_force,omitzero"`
	Quantity      decimal.NullDecimal `json:"quantity,omitzero"`
	LimitPrice    decimal.NullDecimal `json:"limit_price,omitzero"`
	StopPrice     decimal.NullDecimal `json:"stop_price,omitzero"`

	TrailingType     TrailingType        `json:"trailing_type,omitzero"`
	TrailingStopStep decimal.NullDecimal `json:"trailing_stop_step,omitzero"`
	TargetVolPercent decimal.NullDecimal `json:"target_vol_percent,omitzero"`
	MaxTargetPercent decimal.NullDecimal `json:"max_target_percent,omitzero"`
	AlgoStartTime    string              `json:"algo_start_time,omitzero"`
	AlgoEndTime      string              `json:"algo_end_time,omitzero"`

	// Legs modifies option leg quantities by leg ID, as returned by Order.
	Legs []LegModification `json:"legs,omitempty"`
}

// LegModification changes the quantity of one option leg. Both fields are
// required.
type LegModification struct {
	ID       string              `json:"id"`
	Quantity decimal.NullDecimal `json:"quantity,omitzero"`
}

// ReplaceOrder modifies a working order.
func (c *Client) ReplaceOrder(ctx context.Context, accountID string, mod OrderModification) (*OrderReceipt, error) {
	if mod.ClientOrderID == "" {
		return nil, fmt.Errorf("%w: ClientOrderID is required", ErrInvalidOrder)
	}
	for _, d := range []*decimal.NullDecimal{&mod.Quantity, &mod.LimitPrice, &mod.StopPrice, &mod.TrailingStopStep,
		&mod.TargetVolPercent, &mod.MaxTargetPercent} {
		clear(d)
	}
	if len(mod.Legs) == 0 {
		mod.Legs = nil
	}
	for i, leg := range mod.Legs {
		if leg.ID == "" || !leg.Quantity.Valid {
			return nil, fmt.Errorf("%w: leg %d needs ID and Quantity", ErrInvalidOrder, i)
		}
	}
	body := struct {
		AccountID    string              `json:"account_id"`
		ModifyOrders []OrderModification `json:"modify_orders"`
	}{accountID, []OrderModification{mod}}
	var out OrderReceipt
	if err := c.post(ctx, "/trading/orders/replace", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CancelOrder cancels a working order by its client order ID.
func (c *Client) CancelOrder(ctx context.Context, accountID, clientOrderID string) (*OrderReceipt, error) {
	if clientOrderID == "" {
		return nil, fmt.Errorf("%w: clientOrderID is required", ErrInvalidOrder)
	}
	body := struct {
		AccountID     string `json:"account_id"`
		ClientOrderID string `json:"client_order_id"`
	}{accountID, clientOrderID}
	var out OrderReceipt
	if err := c.post(ctx, "/trading/orders/cancel", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OrderGroup is how Webull reports orders: a combo envelope containing one
// order for a Normal order and several for OCO, OTO and OTOCO groups.
type OrderGroup struct {
	ClientOrderID string      `json:"client_order_id"`
	ComboType     ComboType   `json:"combo_type"`
	ComboOrderID  string      `json:"combo_order_id"`
	Orders        []OrderInfo `json:"orders"`
}

// OrderInfo is the state of one order.
type OrderInfo struct {
	ClientOrderID  string         `json:"client_order_id"`
	OrderID        string         `json:"order_id"`
	Symbol         string         `json:"symbol"`
	Side           Side           `json:"side"`
	Status         OrderStatus    `json:"status"`
	Type           OrderType      `json:"order_type"`
	InstrumentType InstrumentType `json:"instrument_type"`
	TradingSession TradingSession `json:"support_trading_session"`
	EntrustType    EntrustType    `json:"entrust_type"`
	TimeInForce    TimeInForce    `json:"time_in_force"`
	ExpireDate     string         `json:"expire_date"`

	TotalQuantity  decimal.Decimal     `json:"total_quantity"`
	FilledQuantity decimal.NullDecimal `json:"filled_quantity"`
	// FilledPrice is the average fill price and is unset until a fill.
	FilledPrice      decimal.NullDecimal `json:"filled_price"`
	LimitPrice       decimal.NullDecimal `json:"limit_price"`
	StopPrice        decimal.NullDecimal `json:"stop_price"`
	TrailingType     TrailingType        `json:"trailing_type"`
	TrailingStopStep decimal.NullDecimal `json:"trailing_stop_step"`

	// PlaceTime and FilledTime are UTC. FilledTime is zero until a fill.
	PlaceTime  time.Time `json:"place_time_at"`
	FilledTime time.Time `json:"filled_time_at,omitzero"`

	AlgoType         AlgoType            `json:"algo_type"`
	TargetVolPercent decimal.NullDecimal `json:"target_vol_percent"`
	MaxTargetPercent decimal.NullDecimal `json:"max_target_percent"`
	AlgoStartTime    string              `json:"algo_start_time"`
	AlgoEndTime      string              `json:"algo_end_time"`

	EventOutcome   EventOutcome   `json:"event_outcome"`
	EventTradeMode EventTradeMode `json:"event_trade_mode"`

	OptionStrategy OptionStrategy `json:"option_strategy"`
	PositionIntent PositionIntent `json:"position_intent"`
	LegDirection   LegDirection   `json:"leg_in_or_out"`
	PositionID     string         `json:"position_id"`
	LegInStrategy  LegInStrategy  `json:"leg_in_strategy"`
	Legs           []OrderInfoLeg `json:"legs"`

	Commission Commission `json:"commission"`
	Fees       []Fee      `json:"fees"`
}

// OrderInfoLeg is the state of one leg of an option order.
type OrderInfoLeg struct {
	ID       string          `json:"id"`
	Symbol   string          `json:"symbol"`
	Side     Side            `json:"side"`
	Quantity decimal.Decimal `json:"quantity"`

	OptionType          OptionType          `json:"option_type"`
	Style               OptionStyle         `json:"option_category"`
	StrikePrice         decimal.NullDecimal `json:"strike_price"`
	ExpireDate          string              `json:"option_expire_date"`
	ContractMultiplier  decimal.NullDecimal `json:"option_contract_multiplier"`
	ContractDeliverable decimal.NullDecimal `json:"option_contract_deliverable"`
}

// Commission is the commission charged on an order.
type Commission struct {
	Actual     decimal.NullDecimal `json:"actual_commission"`
	Receivable decimal.NullDecimal `json:"receivable_commission"`
}

// Fee is one regulatory or exchange fee on an order.
type Fee struct {
	Type       string              `json:"type"`
	Actual     decimal.NullDecimal `json:"actual_value"`
	Receivable decimal.NullDecimal `json:"receivable_value"`
}

// Order retrieves an order by its client order ID.
func (c *Client) Order(ctx context.Context, accountID, clientOrderID string) (*OrderGroup, error) {
	if clientOrderID == "" {
		return nil, fmt.Errorf("%w: clientOrderID is required", ErrInvalidOrder)
	}
	q := params{}
	q.set("account_id", accountID)
	q.set("client_order_id", clientOrderID)
	var out OrderGroup
	if err := c.get(ctx, "/trading/orders/get", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// OrdersRequest pages through open or historical orders.
type OrdersRequest struct {
	AccountID string
	// StartDate and EndDate are yyyy-MM-dd and apply to OrderHistory only.
	StartDate string
	EndDate   string
	// PageSize is between 10 and 100. Zero lets Webull apply its default.
	PageSize int
	// LastClientOrderID is the ClientOrderID of the final group from the
	// previous page. Leave empty for the first page.
	LastClientOrderID string
}

func (r OrdersRequest) params(history bool) params {
	q := params{}
	q.set("account_id", r.AccountID)
	if history {
		q.set("start_date", r.StartDate)
		q.set("end_date", r.EndDate)
	}
	q.setInt("page_size", r.PageSize)
	q.set("last_client_order_id", r.LastClientOrderID)
	return q
}

// OpenOrders returns one page of working orders.
func (c *Client) OpenOrders(ctx context.Context, req OrdersRequest) ([]OrderGroup, error) {
	var out []OrderGroup
	if err := c.get(ctx, "/trading/orders/open-orders/list", req.params(false), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// OrderHistory returns one page of past orders. Its status for a recently
// changed order can lag behind Order; an order reported CANCELLED by Order was
// observed as PENDING here moments later.
func (c *Client) OrderHistory(ctx context.Context, req OrdersRequest) ([]OrderGroup, error) {
	var out []OrderGroup
	if err := c.get(ctx, "/trading/orders/historical-orders/list", req.params(true), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// BatchResult reports the outcome of PlaceOrders.
type BatchResult struct {
	Total   int                `json:"total"`
	Success int                `json:"success"`
	Failed  int                `json:"failed"`
	Orders  []BatchOrderResult `json:"batch_orders"`
}

// BatchOrderResult is the outcome for one order in a batch. Code and Message
// are set when the order was rejected.
type BatchOrderResult struct {
	ClientOrderID string `json:"client_order_id"`
	OrderID       string `json:"order_id"`
	Code          string `json:"error_code"`
	Message       string `json:"message"`
}

// PlaceOrders submits several equity orders at once. Webull accepts only
// Normal combo, Market or Limit, Day orders by quantity in a batch; each order
// is validated as for PlaceOrder and those constraints are enforced locally.
func (c *Client) PlaceOrders(ctx context.Context, accountID string, orders []*Order) (*BatchResult, error) {
	if len(orders) == 0 {
		return nil, fmt.Errorf("%w: no orders", ErrInvalidOrder)
	}
	batch := make([]Order, 0, len(orders))
	for i, o := range orders {
		if err := o.prepare(); err != nil {
			return nil, fmt.Errorf("order %d: %w", i, err)
		}
		switch {
		case o.InstrumentType != InstrumentEquity:
			return nil, fmt.Errorf("%w: order %d: batch orders must be equities", ErrInvalidOrder, i)
		case o.ComboType != Normal:
			return nil, fmt.Errorf("%w: order %d: batch orders must be NORMAL combo", ErrInvalidOrder, i)
		case o.EntrustType != ByQuantity:
			return nil, fmt.Errorf("%w: order %d: batch orders must be by quantity", ErrInvalidOrder, i)
		case o.Type != Market && o.Type != Limit:
			return nil, fmt.Errorf("%w: order %d: batch orders must be MARKET or LIMIT", ErrInvalidOrder, i)
		case o.TimeInForce != Day:
			return nil, fmt.Errorf("%w: order %d: batch orders must be DAY", ErrInvalidOrder, i)
		case o.Side == Short:
			return nil, fmt.Errorf("%w: order %d: batch orders cannot be SHORT", ErrInvalidOrder, i)
		}
		batch = append(batch, o.forPlacement())
	}
	body := struct {
		AccountID   string  `json:"account_id"`
		BatchOrders []Order `json:"batch_orders"`
	}{accountID, batch}
	var out BatchResult
	if err := c.post(ctx, "/trading/orders/batch-place", body, &out); err != nil {
		return nil, classify(err)
	}
	return &out, nil
}
