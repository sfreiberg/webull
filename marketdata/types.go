package marketdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Category identifies an asset class in market-data requests.
type Category string

// Categories accepted by the stock endpoints.
const (
	USStock Category = "US_STOCK"
	USETF   Category = "US_ETF"
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
// buyer- and seller-initiated and N is neutral.
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
// means the field was absent.
type Millis struct{ time.Time }

// UnmarshalJSON accepts a number, a numeric string, or null.
func (m *Millis) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		m.Time = time.Time{}
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("marketdata: %q is not an epoch-millisecond time", s)
	}
	m.Time = time.UnixMilli(n).UTC()
	return nil
}

// MarshalJSON writes epoch milliseconds as a number, or null when zero.
func (m Millis) MarshalJSON() ([]byte, error) {
	if m.IsZero() {
		return []byte("null"), nil
	}
	return []byte(strconv.FormatInt(m.UnixMilli(), 10)), nil
}

// isoLayout is Webull's ISO 8601 form: an offset without a colon, which
// time.RFC3339 rejects.
const isoLayout = "2006-01-02T15:04:05.000-0700"

// Time is a point in time transmitted in Webull's ISO 8601 form, such as
// "2026-08-27T04:00:00.000+0000". The zero value means the field was absent.
type Time struct{ time.Time }

// UnmarshalJSON accepts Webull's ISO form, RFC 3339, or null.
func (t *Time) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		t.Time = time.Time{}
		return nil
	}
	for _, layout := range []string{isoLayout, time.RFC3339Nano} {
		if parsed, err := time.Parse(layout, s); err == nil {
			t.Time = parsed.UTC()
			return nil
		}
	}
	return fmt.Errorf("marketdata: %q is not a recognised time", s)
}

// MarshalJSON writes Webull's ISO form, or null when zero.
func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.UTC().Format(isoLayout))
}

// ErrNotSubscribed is returned when the key is not entitled to the data
// requested. The wrapped *webull.APIError's Message names the product.
var ErrNotSubscribed = errors.New("marketdata: not subscribed to this data")

// notSubscribedCode is Webull's code for a missing market-data entitlement.
const notSubscribedCode = "MARKET_DATA_NOT_SUBSCRIBED"

// classify wraps errors this package gives a name to.
func classify(err error) error {
	var coded interface{ ErrorCode() string }
	if errors.As(err, &coded) && coded.ErrorCode() == notSubscribedCode {
		return fmt.Errorf("%w: %w", ErrNotSubscribed, err)
	}
	return err
}
