package trade

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestPlaceOrdersRejectsNonNormalOrAmount(t *testing.T) {
	c, f := newBodyClient(t, "", "order_place.json")
	for name, bad := range map[string]*Order{
		"oco":    {Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1"), ComboType: OCO},
		"amount": {Symbol: "AAPL", Side: Buy, Type: Market, TotalCashAmount: Price("100")},
	} {
		if _, err := c.PlaceOrders(context.Background(), "A", []*Order{bad}); !errors.Is(err, ErrInvalidOrder) {
			t.Errorf("%s: got %v", name, err)
		}
	}
	if f.gotRaw != nil {
		t.Error("invalid batch reached the network")
	}
}

func TestOrderLookupRejectsEmptyID(t *testing.T) {
	c, f := newTestClient(t, "", "order_get_cancelled.json")
	if _, err := c.Order(context.Background(), "A", ""); !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("got %v", err)
	}
	if f.gotPath != "" {
		t.Error("request was sent with an empty client order id")
	}
}

func TestEmptyLegsAreOmitted(t *testing.T) {
	o := Order{Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1"), Legs: []OrderLeg{}}
	if err := o.prepare(); err != nil {
		t.Fatal(err)
	}
	if o.InstrumentType != InstrumentEquity {
		t.Errorf("empty legs classified as %s", o.InstrumentType)
	}
	if b, _ := json.Marshal(o); strings.Contains(string(b), "legs") {
		t.Errorf("empty legs serialised: %s", b)
	}
	c, f := newBodyClient(t, "", "order_place.json")
	_, _ = c.ReplaceOrder(context.Background(), "A", OrderModification{ClientOrderID: "clientorder0001", Legs: []LegModification{}})
	if strings.Contains(string(f.gotRaw), "legs") {
		t.Errorf("empty modification legs serialised: %s", f.gotRaw)
	}
}

func TestLegModificationRequiresQuantity(t *testing.T) {
	c, f := newBodyClient(t, "", "order_place.json")
	_, err := c.ReplaceOrder(context.Background(), "A", OrderModification{
		ClientOrderID: "clientorder0001", Legs: []LegModification{{ID: "leg1"}},
	})
	if !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("a leg without a quantity must be rejected locally, got %v", err)
	}
	if f.gotRaw != nil {
		t.Error("request was sent")
	}
}

func TestPreviewOnlyFieldsAreNotPlaced(t *testing.T) {
	c, f := newBodyClient(t, "/trading/orders/place", "order_place.json")
	o := &Order{Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1"), CurrentAsk: Price("181"), CurrentBid: Price("180")}
	if _, err := c.PlaceOrder(context.Background(), "A", o); err != nil {
		t.Fatal(err)
	}
	sent := firstOrder(t, f.gotBody, "new_orders")
	if _, has := sent["current_ask"]; has {
		t.Error("current_ask was forwarded to place")
	}
	if !o.CurrentAsk.Valid {
		t.Error("the caller's Order must not be modified")
	}
}

func TestUnsetNullDecimalWithStalePayloadIsOmitted(t *testing.T) {
	// Unmarshalling null keeps the old payload with Valid false; prepare must
	// still omit it rather than sending an explicit null.
	o := Order{Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1"),
		StopPrice: decimal.NullDecimal{Decimal: decimal.NewFromInt(5), Valid: false}}
	if err := o.prepare(); err != nil {
		t.Fatal(err)
	}
	if b, _ := json.Marshal(o); strings.Contains(string(b), "stop_price") {
		t.Errorf("stale unset value serialised: %s", b)
	}
	mod := OrderModification{ClientOrderID: "clientorder0001",
		LimitPrice: decimal.NullDecimal{Decimal: decimal.NewFromInt(5), Valid: false}}
	c, f := newBodyClient(t, "", "order_place.json")
	_, _ = c.ReplaceOrder(context.Background(), "A", mod)
	if strings.Contains(string(f.gotRaw), "limit_price") {
		t.Errorf("stale unset value serialised in replace: %s", f.gotRaw)
	}
}

type codedError struct{ code string }

func (e *codedError) Error() string     { return e.code }
func (e *codedError) ErrorCode() string { return e.code }

func TestDuplicateClientOrderIDIsClassified(t *testing.T) {
	c := newClientFor(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusExpectationFailed)
	}))
	c.doer.DecodeError = func(transportResponse) error { return &codedError{code: duplicateOrderCode} }
	_, err := c.PlaceOrder(context.Background(), "A", &Order{Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1")})
	if !errors.Is(err, ErrDuplicateOrder) {
		t.Fatalf("got %v, want ErrDuplicateOrder", err)
	}
	var coded *codedError
	if !errors.As(err, &coded) {
		t.Error("the original error must remain reachable")
	}
}

func TestLegFromSymbolUsesRoot(t *testing.T) {
	leg, err := LegFromSymbol("SPXW260930C03000000")
	if err != nil {
		t.Fatal(err)
	}
	if leg.Symbol != "SPXW" {
		t.Errorf("Symbol = %q; Webull rejects the underlying SPX for weeklies", leg.Symbol)
	}
	if !leg.StrikePrice.Decimal.Equal(decimal.NewFromInt(3000)) {
		t.Errorf("strike = %v", leg.StrikePrice)
	}
}
