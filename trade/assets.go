package trade

import (
	"fmt"
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
		// Single-leg options do not support market or trailing orders. A
		// multi-leg strategy may be a market order - Webull's own covered
		// stock example is one - and validateAsset widens the rule for those.
		orderTypes: []OrderType{Limit, StopLoss, StopLossLimit},
		tifs:       []TimeInForce{Day, GTC},
		entrust:    []EntrustType{ByQuantity},
		maxScale:   0,
		combos:     true, // single-leg only; Combo enforces that
	},
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

func contains[T comparable](xs []T, x T) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// validateAsset appends the problems with o under its instrument's rule.
func (o *Order) validateAsset(problems []string) []string {
	rule, ok := assetRules[o.InstrumentType]
	if !ok {
		return append(problems, fmt.Sprintf("unknown instrument type %q", o.InstrumentType))
	}
	if o.Side != "" && !contains(rule.sides, o.Side) {
		problems = append(problems, fmt.Sprintf("%s orders do not support side %s", o.InstrumentType, o.Side))
	}
	multiLeg := o.InstrumentType == InstrumentOption && o.OptionStrategy != "" && o.OptionStrategy != StrategySingle
	marketStrategy := multiLeg && o.Type == Market
	if o.Type != "" && !contains(rule.orderTypes, o.Type) && !marketStrategy {
		problems = append(problems, fmt.Sprintf("%s orders do not support order type %s", o.InstrumentType, o.Type))
	}
	if !contains(rule.tifs, o.TimeInForce) {
		problems = append(problems, fmt.Sprintf("%s orders do not support time in force %s", o.InstrumentType, o.TimeInForce))
	}
	if !contains(rule.entrust, o.EntrustType) {
		problems = append(problems, fmt.Sprintf("%s orders do not support %s entrust type", o.InstrumentType, o.EntrustType))
	}
	if rule.maxScale >= 0 && o.Quantity.Valid && scale(o.Quantity.Decimal) > rule.maxScale {
		problems = append(problems, fmt.Sprintf("%s quantity allows at most %d decimal places", o.InstrumentType, rule.maxScale))
	}
	if rule.needOutcome && o.EventOutcome == "" {
		problems = append(problems, "event contract orders need an EventOutcome")
	}
	if !rule.needOutcome && o.EventOutcome != "" {
		problems = append(problems, "EventOutcome applies only to event contract orders")
	}
	// Option sells are DAY only; GTC is buy-side only.
	if o.InstrumentType == InstrumentOption && o.Side != Buy && o.TimeInForce != Day {
		problems = append(problems, "option sell orders support only DAY time in force")
	}
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
