package trade

import (
	"fmt"
	"slices"
	"strings"

	"github.com/shopspring/decimal"
)

// assetRule is what Webull permits for one instrument type. The table comes
// from Webull's trading FAQ and asset-class guides rather than its OpenAPI
// definition, which does not express these limits; each rule was checked
// against the sandbox where it could be.
type assetRule struct {
	sides       []Side
	orderTypes  []OrderType
	tifs        []TimeInForce
	entrust     []EntrustType
	maxScale    int32 // decimal places permitted in Quantity; -1 for any
	needOutcome bool  // EventOutcome is required
	combos      bool  // may be part of a Combo
}

var assetRules = map[InstrumentType]assetRule{
	InstrumentEquity: {
		sides:      []Side{Buy, Sell, Short},
		orderTypes: []OrderType{Market, Limit, StopLoss, StopLossLimit, TrailingStopLoss, MarketOnOpen, MarketOnClose, LimitOnOpen},
		tifs:       []TimeInForce{Day, GTC, IOC, GTD, FOK},
		entrust:    []EntrustType{ByQuantity, ByAmount},
		maxScale:   -1,
		combos:     true,
	},
	InstrumentOption: {
		sides: []Side{Buy, Sell, Short},
		// Webull's FAQ says options do not support market orders. The
		// sandbox previews market orders on single-leg and multi-leg option
		// orders alike, so that is not enforced here. Trailing stops are
		// rejected by the server ("invalid trailing_type") and are excluded.
		orderTypes: []OrderType{Market, Limit, StopLoss, StopLossLimit},
		// IOC and FOK are rejected by the server ("invalid time_in_force").
		// GTD was rejected on its expiry date rather than its time in force,
		// which is inconclusive, and is excluded on the FAQ's word.
		tifs:     []TimeInForce{Day, GTC},
		entrust:  []EntrustType{ByQuantity},
		maxScale: 0,
		combos:   true, // single-leg only; Order.prepare enforces that
	},
	// Futures, crypto and event contracts cannot be grouped: the sandbox
	// rejects a bracket on any of them with "Inconsistent instrument type in
	// combo".
	InstrumentFutures: {
		sides:      []Side{Buy, Sell},
		orderTypes: []OrderType{Market, Limit, StopLoss, StopLossLimit, TrailingStopLoss},
		tifs:       []TimeInForce{Day, GTC},
		entrust:    []EntrustType{ByQuantity},
		maxScale:   0,
	},
	InstrumentCrypto: {
		sides:      []Side{Buy, Sell},
		orderTypes: []OrderType{Market, Limit, StopLossLimit},
		tifs:       []TimeInForce{Day, GTC, IOC},
		entrust:    []EntrustType{ByQuantity, ByAmount},
		maxScale:   8,
	},
	InstrumentEvent: {
		sides: []Side{Buy, Sell},
		// Only limit orders. Webull's guide also says DAY only, but the
		// sandbox places GTC event orders and its OpenAPI definition lists
		// every time in force, so that is not restricted here.
		orderTypes:  []OrderType{Limit},
		tifs:        []TimeInForce{Day, GTC, IOC, GTD, FOK},
		entrust:     []EntrustType{ByQuantity},
		maxScale:    2,
		needOutcome: true,
	},
}

// Values this package knows. A value outside these sets is one Webull added
// after this release; it is forwarded for the server to judge rather than
// rejected locally, so the enumerations stay open.
var (
	knownSides   = []Side{Buy, Sell, Short}
	knownTypes   = []OrderType{Market, Limit, StopLoss, StopLossLimit, TrailingStopLoss, MarketOnOpen, MarketOnClose, LimitOnOpen}
	knownTIFs    = []TimeInForce{Day, GTC, IOC, GTD, FOK}
	knownEntrust = []EntrustType{ByQuantity, ByAmount}
)

// validateAsset appends the problems with o under its instrument's rule.
// An instrument type this package does not know is left for the server.
func (o *Order) validateAsset(problems []string) []string {
	rule, ok := assetRules[o.InstrumentType]
	if !ok {
		return problems
	}
	if slices.Contains(knownSides, o.Side) && !slices.Contains(rule.sides, o.Side) {
		problems = append(problems, fmt.Sprintf("%s orders do not support side %s", o.InstrumentType, o.Side))
	}
	if slices.Contains(knownTypes, o.Type) && !slices.Contains(rule.orderTypes, o.Type) {
		problems = append(problems, fmt.Sprintf("%s orders do not support order type %s", o.InstrumentType, o.Type))
	}
	if slices.Contains(knownTIFs, o.TimeInForce) && !slices.Contains(rule.tifs, o.TimeInForce) {
		problems = append(problems, fmt.Sprintf("%s orders do not support time in force %s", o.InstrumentType, o.TimeInForce))
	}
	if slices.Contains(knownEntrust, o.EntrustType) && !slices.Contains(rule.entrust, o.EntrustType) {
		problems = append(problems, fmt.Sprintf("%s orders do not support %s entrust type", o.InstrumentType, o.EntrustType))
	}
	if rule.maxScale >= 0 && o.Quantity.Valid && scale(o.Quantity.Decimal) > rule.maxScale {
		problems = append(problems, fmt.Sprintf("%s quantity allows at most %d decimal places", o.InstrumentType, rule.maxScale))
	}
	// Option legs are whole contracts too; the server rejects "1.5". A leg
	// with no InstrumentType yet is an option leg, the default.
	for i, leg := range o.Legs {
		isOption := leg.InstrumentType == InstrumentOption || leg.InstrumentType == ""
		if isOption && leg.Quantity.Valid && scale(leg.Quantity.Decimal) > 0 {
			problems = append(problems, fmt.Sprintf("leg %d: option quantity must be whole contracts", i))
		}
	}
	if rule.needOutcome && o.EventOutcome == "" {
		problems = append(problems, "event contract orders need an EventOutcome")
	}
	if !rule.needOutcome && o.EventOutcome != "" {
		problems = append(problems, "EventOutcome applies only to event contract orders")
	}
	// Webull's FAQ also says option sells are DAY only. The sandbox previews
	// a GTC option sell, so that is not enforced here either; the server
	// remains the authority at placement.
	if o.ComboType != Normal && !rule.combos {
		problems = append(problems, fmt.Sprintf("%s orders cannot be part of a combo", o.InstrumentType))
	}
	return problems
}

// scale returns the number of decimal places d carries. Decimal's String
// form drops trailing zeros, so "1.50" counts as one.
func scale(d decimal.Decimal) int32 {
	s := d.String()
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return int32(len(s) - i - 1)
	}
	return 0
}
