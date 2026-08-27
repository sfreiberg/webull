package trade

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func TestAssetRulesReject(t *testing.T) {
	q := Price("1")
	cases := map[string]Order{
		"option trailing":    {Symbol: "AAPL", Side: Buy, Type: TrailingStopLoss, Quantity: q, TrailingType: TrailByAmount, TrailingStopStep: q, Legs: []OrderLeg{{Symbol: "AAPL", OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("240")}}},
		"option fractional":  {Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1.5"), LimitPrice: q, Legs: []OrderLeg{{Symbol: "AAPL", OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("240")}}},
		"futures short":      {InstrumentType: InstrumentFutures, Symbol: "ESZ6", Side: Short, Type: Limit, Quantity: q, LimitPrice: q},
		"futures ioc":        {InstrumentType: InstrumentFutures, Symbol: "ESZ6", Side: Buy, Type: Limit, Quantity: q, LimitPrice: q, TimeInForce: IOC},
		"futures amount":     {InstrumentType: InstrumentFutures, Symbol: "ESZ6", Side: Buy, Type: Market, TotalCashAmount: q},
		"futures fractional": {InstrumentType: InstrumentFutures, Symbol: "ESZ6", Side: Buy, Type: Market, Quantity: Price("0.5")},
		"crypto stop":        {InstrumentType: InstrumentCrypto, Symbol: "BTCUSD", Side: Buy, Type: StopLoss, Quantity: q, StopPrice: q},
		"crypto trailing":    {InstrumentType: InstrumentCrypto, Symbol: "BTCUSD", Side: Buy, Type: TrailingStopLoss, Quantity: q, TrailingType: TrailByAmount, TrailingStopStep: q},
		"crypto gtd":         {InstrumentType: InstrumentCrypto, Symbol: "BTCUSD", Side: Buy, Type: Limit, Quantity: q, LimitPrice: q, TimeInForce: GTD, ExpireDate: "2026-12-01"},
		"crypto too precise": {InstrumentType: InstrumentCrypto, Symbol: "BTCUSD", Side: Buy, Type: Limit, Quantity: Price("0.000000001"), LimitPrice: q},
		"event market":       {InstrumentType: InstrumentEvent, Symbol: "KX-T6", Side: Buy, Type: Market, Quantity: q, EventOutcome: OutcomeYes},
		"event no outcome":   {InstrumentType: InstrumentEvent, Symbol: "KX-T6", Side: Buy, Type: Limit, Quantity: q, LimitPrice: q},
		"event short":        {InstrumentType: InstrumentEvent, Symbol: "KX-T6", Side: Short, Type: Limit, Quantity: q, LimitPrice: q, EventOutcome: OutcomeNo},
		"event amount":       {InstrumentType: InstrumentEvent, Symbol: "KX-T6", Side: Buy, Type: Limit, LimitPrice: q, TotalCashAmount: q, EventOutcome: OutcomeYes},
		"event too precise":  {InstrumentType: InstrumentEvent, Symbol: "KX-T6", Side: Buy, Type: Limit, Quantity: Price("1.005"), LimitPrice: q, EventOutcome: OutcomeYes},
		"outcome on equity":  {Symbol: "AAPL", Side: Buy, Type: Market, Quantity: q, EventOutcome: OutcomeYes},
		"unknown instrument": {InstrumentType: "BOND", Symbol: "X", Side: Buy, Type: Market, Quantity: q},
		"futures in a combo": {InstrumentType: InstrumentFutures, Symbol: "ESZ6", Side: Buy, Type: Limit, Quantity: q, LimitPrice: q, ComboType: RoleMaster},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			if err := o.prepare(); !errors.Is(err, ErrInvalidOrder) {
				t.Errorf("got %v, want ErrInvalidOrder", err)
			}
		})
	}
}

func TestAssetRulesAccept(t *testing.T) {
	q := Price("1")
	leg := OrderLeg{Symbol: "AAPL", OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("240")}
	cases := map[string]Order{
		"option buy gtc":  {Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: q, LimitPrice: q, TimeInForce: GTC, Legs: []OrderLeg{leg}},
		"option market":   {Symbol: "AAPL", Side: Buy, Type: Market, Quantity: q, Legs: []OrderLeg{leg}}, // FAQ says no; the sandbox previews it
		"option sell gtc": {Symbol: "AAPL", Side: Sell, Type: Limit, Quantity: q, LimitPrice: q, TimeInForce: GTC, Legs: []OrderLeg{leg}},
		"option sell day": {Symbol: "AAPL", Side: Sell, Type: Limit, Quantity: q, LimitPrice: q, Legs: []OrderLeg{leg}},
		"covered stock market": {Symbol: "AAPL", Side: Buy, Type: Market, Quantity: q, OptionStrategy: StrategyCoveredStock,
			Legs: []OrderLeg{{Symbol: "AAPL", Side: Buy, Quantity: Price("100"), InstrumentType: InstrumentEquity}, {Symbol: "AAPL", Side: Sell, OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("260")}}},
		"futures limit gtc": {InstrumentType: InstrumentFutures, Symbol: "ESZ6", Side: Buy, Type: Limit, Quantity: q, LimitPrice: q, TimeInForce: GTC},
		"futures trailing":  {InstrumentType: InstrumentFutures, Symbol: "ESZ6", Side: Sell, Type: TrailingStopLoss, Quantity: q, TrailingType: TrailByAmount, TrailingStopStep: q},
		"crypto ioc":        {InstrumentType: InstrumentCrypto, Symbol: "BTCUSD", Side: Buy, Type: Limit, Quantity: Price("0.00000001"), LimitPrice: q, TimeInForce: IOC},
		"crypto by amount":  {InstrumentType: InstrumentCrypto, Symbol: "BTCUSD", Side: Buy, Type: Market, TotalCashAmount: Price("5")},
		"crypto stop limit": {InstrumentType: InstrumentCrypto, Symbol: "BTCUSD", Side: Sell, Type: StopLossLimit, Quantity: q, LimitPrice: q, StopPrice: q},
		"event limit day":   {InstrumentType: InstrumentEvent, Symbol: "KX-T6", Side: Buy, Type: Limit, Quantity: Price("1.25"), LimitPrice: Price("0.10"), EventOutcome: OutcomeYes},
		"event gtc":         {InstrumentType: InstrumentEvent, Symbol: "KX-T6", Side: Sell, Type: Limit, Quantity: q, LimitPrice: Price("0.90"), TimeInForce: GTC, EventOutcome: OutcomeNo},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			if err := o.prepare(); err != nil {
				t.Errorf("valid order rejected: %v", err)
			}
		})
	}
	if o := cases["futures limit gtc"]; o.TradingSession != "" {
		t.Error("the equity session default must not leak onto futures")
	}
}

func TestQuantityScale(t *testing.T) {
	for in, want := range map[string]int32{"1": 0, "1.0": 0, "1.50": 1, "0.001": 3, "0.00000001": 8, "100": 0, "12.345": 3} {
		if got := scale(decimal.RequireFromString(in)); got != want {
			t.Errorf("scale(%s) = %d, want %d", in, got, want)
		}
	}
}
