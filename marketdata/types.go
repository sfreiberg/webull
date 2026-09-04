package marketdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sfreiberg/webull/internal/transport"
	"github.com/sfreiberg/webull/internal/wire"
)

// Category identifies an asset class in market-data requests. It is an
// alias of the shared wire type also used by the streaming package.
type Category = wire.Category

// Decimal is the SDK's fixed-point decimal type. It embeds
// github.com/shopspring/decimal.Decimal, so all of that type's methods are
// available, and additionally decodes Webull's absent forms — null and an
// empty string — where shopspring's own type rejects "".
type Decimal = wire.Decimal

// NullDecimal is the SDK's nullable fixed-point decimal type, the counterpart
// of Decimal for fields that may be absent.
type NullDecimal = wire.NullDecimal

// Categories. USStock and USETF are accepted by the stock endpoints; the rest
// each have their own.
const (
	USStock   Category = "US_STOCK"
	USETF     Category = "US_ETF"
	USOption  Category = "US_OPTION"
	USFutures Category = "US_FUTURES"
	USCrypto  Category = "US_CRYPTO"
	USEvent   Category = "US_EVENT"
)

// Timespan is a bar interval.
type Timespan string

// Bar intervals.
const (
	Minute    Timespan = "M1"
	Minute5   Timespan = "M5"
	Minute15  Timespan = "M15"
	Minute30  Timespan = "M30"
	Minute60  Timespan = "M60"
	Minute120 Timespan = "M120"
	Minute240 Timespan = "M240"
	Daily     Timespan = "D"
	Weekly    Timespan = "W"
	Monthly   Timespan = "M"
	Yearly    Timespan = "Y"
	Second5   Timespan = "S5"  // footprints only
	Second15  Timespan = "S15" // footprints only
)

// TradingSession selects a part of the trading day.
type TradingSession string

// Trading sessions.
const (
	PreMarket  TradingSession = "PRE"
	Regular    TradingSession = "RTH"
	AfterHours TradingSession = "ATH"
	Overnight  TradingSession = "OVN"
)

// TickSide is the aggressor side of a trade, as Webull reports it. The API
// documents the letters B, S, G, L and N without defining them; B and S are
// buyer- and seller-initiated and N is neutral. Option ticks also carry an
// undocumented "NS".
type TickSide string

// Tick sides.
const (
	TickBuy     TickSide = "B"
	TickSell    TickSide = "S"
	TickNeutral TickSide = "N"
)

// ImbalanceType selects the opening or closing auction imbalance.
type ImbalanceType string

// Imbalance types.
const (
	PreOpen  ImbalanceType = "PRE_OPEN"
	PreClose ImbalanceType = "PRE_CLOSE"
)

// Millis is a point in time transmitted as epoch milliseconds, which Webull
// sends sometimes as a JSON number and sometimes as a string. The zero value
// means the field was absent; a wire value of 0 is treated the same way,
// since no market data event happened at the epoch. It is an alias of the
// shared wire type also used by the root package.
type Millis = wire.Millis

// Time is a point in time transmitted in Webull's ISO 8601 form, such as
// "2026-08-27T04:00:00.000+0000". It accepts every date form the API uses;
// the zero value means the field was absent. It is an alias of the shared
// wire type also used by the events package.
type Time = wire.Time

// ErrNotSubscribed is returned when the key is not entitled to the data
// requested. The wrapped *webull.APIError's Message names the product. Because
// the response is an HTTP 403, the same error also matches webull.ErrPermission.
var ErrNotSubscribed = errors.New("marketdata: not subscribed to this data")

// notSubscribedCode is Webull's code for a missing market-data entitlement.
const notSubscribedCode = "MARKET_DATA_NOT_SUBSCRIBED"

// classify wraps errors this package gives a name to.
//
// An entitlement failure is an HTTP 403, so it also matches
// webull.ErrPermission. Check ErrNotSubscribed first when the distinction
// matters.
func classify(err error) error {
	if transport.HasCode(err, notSubscribedCode) {
		return fmt.Errorf("%w: %w", ErrNotSubscribed, err)
	}
	return err
}

// barsList decodes a bars response in either of the shapes Webull uses: the
// bare array the sandbox returns for options, futures, crypto and events, or
// the {"result": [...]} envelope its documentation shows and the stock
// endpoint returns.
type barsList []Bars

func (l *barsList) UnmarshalJSON(b []byte) error {
	trimmed := strings.TrimSpace(string(b))
	if strings.HasPrefix(trimmed, "[") {
		var bare []Bars
		if err := json.Unmarshal(b, &bare); err != nil {
			return err
		}
		*l = bare
		return nil
	}
	var wrapped struct {
		Result []Bars `json:"result"`
	}
	if err := json.Unmarshal(b, &wrapped); err != nil {
		return err
	}
	*l = wrapped.Result
	return nil
}
