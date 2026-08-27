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

func buyLimit(price string) *Order {
	return &Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: Price(price)}
}
func sellLimit(price string) *Order {
	return &Order{Symbol: "AAPL", Side: Sell, Type: Limit, Quantity: Price("1"), LimitPrice: Price(price)}
}
func sellStop(price string) *Order {
	return &Order{Symbol: "AAPL", Side: Sell, Type: StopLoss, Quantity: Price("1"), StopPrice: Price(price)}
}

func TestBracketAssignsRolesAndValidates(t *testing.T) {
	c := Bracket(buyLimit("1.00"), sellLimit("999"), sellStop("0.50"))
	if err := c.prepare(); err != nil {
		t.Fatal(err)
	}
	if c.ClientComboOrderID == "" {
		t.Error("combo id not generated")
	}
	roles := []ComboType{c.Orders[0].ComboType, c.Orders[1].ComboType, c.Orders[2].ComboType}
	if roles[0] != RoleMaster || roles[1] != RoleStopProfit || roles[2] != RoleStopLoss {
		t.Errorf("roles = %v", roles)
	}
	ids := map[string]bool{}
	for _, o := range c.Orders {
		if o.ClientOrderID == "" || ids[o.ClientOrderID] {
			t.Errorf("client order ids must be generated and distinct: %+v", o.ClientOrderID)
		}
		ids[o.ClientOrderID] = true
	}
}

func TestBracketWithoutMasterClosesAPosition(t *testing.T) {
	c := Bracket(nil, sellLimit("999"), sellStop("0.50"))
	if err := c.prepare(); err != nil {
		t.Fatalf("exits-only bracket should be valid: %v", err)
	}
	if err := Bracket(nil, nil, nil).prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("an empty bracket must be rejected")
	}
	if err := Bracket(buyLimit("1"), nil, nil).prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("a master with no exits is not a bracket")
	}
}

func TestComboRoleRules(t *testing.T) {
	cases := map[string]*Combo{
		"take profit must be limit": Bracket(buyLimit("1"), sellStop("999"), nil),
		"stop loss must be stop":    Bracket(buyLimit("1"), nil, sellLimit("0.5")),
		"bracket master not stop":   Bracket(sellStop("1"), sellLimit("999"), nil),
		"oco needs two":             OCO(buyLimit("1")),
		"oco no market":             OCO(buyLimit("1"), &Order{Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1")}),
		"oto too many":              OTO(buyLimit("1"), buyLimit("1"), buyLimit("1"), buyLimit("1"), buyLimit("1"), buyLimit("1"), buyLimit("1"), buyLimit("1")),
		"otoco no market children":  OTOCO(buyLimit("1"), &Order{Symbol: "AAPL", Side: Sell, Type: Market, Quantity: Price("1")}),
		"oto equity only": OTO(buyLimit("1"), &Order{Symbol: "AAPL", Side: Sell, Type: Limit, Quantity: Price("1"), LimitPrice: Price("1"),
			Legs: []OrderLeg{{Symbol: "AAPL", OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("240")}}}),
		"multi-leg cannot bracket": Bracket(&Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: Price("1"),
			OptionStrategy: StrategyVertical, Legs: []OrderLeg{
				{Symbol: "AAPL", Side: Buy, OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("240")},
				{Symbol: "AAPL", Side: Sell, OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("250")}}},
			sellLimit("999"), nil),
		"empty": {},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := c.prepare(); !errors.Is(err, ErrInvalidOrder) {
				t.Errorf("got %v, want ErrInvalidOrder", err)
			}
		})
	}
}

func TestComboValidShapes(t *testing.T) {
	single := &Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: Price("1"),
		Legs: []OrderLeg{{Symbol: "AAPL", OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("240")}}}
	for name, c := range map[string]*Combo{
		"bracket":         Bracket(buyLimit("1"), sellLimit("999"), sellStop("0.5")),
		"bracket tp only": Bracket(buyLimit("1"), sellLimit("999"), nil),
		"option bracket":  Bracket(single, sellLimit("999"), nil),
		"oto":             OTO(buyLimit("1"), sellLimit("999")),
		"oto six":         OTO(buyLimit("1"), sellLimit("9"), sellLimit("9"), sellLimit("9"), sellLimit("9"), sellLimit("9"), sellLimit("9")),
		"oco":             OCO(buyLimit("1"), sellStop("999")),
		"otoco":           OTOCO(buyLimit("1"), sellLimit("999"), sellStop("0.5")),
	} {
		t.Run(name, func(t *testing.T) {
			if err := c.prepare(); err != nil {
				t.Errorf("valid shape rejected: %v", err)
			}
		})
	}
}

func TestConstructorsToleratedNilOrders(t *testing.T) {
	for name, c := range map[string]*Combo{
		"oto nil master":   OTO(nil, sellLimit("9")),
		"oto nil child":    OTO(buyLimit("1"), nil),
		"oco nil":          OCO(buyLimit("1"), nil),
		"otoco nil master": OTOCO(nil, sellLimit("9")),
	} {
		t.Run(name, func(t *testing.T) {
			if err := c.prepare(); !errors.Is(err, ErrInvalidOrder) {
				t.Errorf("got %v, want ErrInvalidOrder rather than a panic", err)
			}
		})
	}
}

func TestBracketExitsMustCloseTheMaster(t *testing.T) {
	sameSide := Bracket(buyLimit("1"), buyLimit("9"), nil)
	if err := sameSide.prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("a take-profit on the same side as the master must be rejected")
	}
	other := sellLimit("9")
	other.Symbol = "MSFT"
	if err := Bracket(buyLimit("1"), other, nil).prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("an exit on a different symbol must be rejected")
	}
	shortMaster := &Order{Symbol: "AAPL", Side: Short, Type: Limit, Quantity: Price("1"), LimitPrice: Price("999")}
	if err := Bracket(shortMaster, buyLimit("1"), nil).prepare(); err != nil {
		t.Errorf("a buy exit closes a short: %v", err)
	}
}

func TestComboIDLengthIsValidated(t *testing.T) {
	c := Bracket(buyLimit("1"), sellLimit("9"), nil)
	c.ClientComboOrderID = "short"
	if err := c.prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("a combo id outside 10-40 characters must be rejected")
	}
}

func TestPreviewComboKeepsPreviewOnlyFields(t *testing.T) {
	c, f := newBodyClient(t, "/trading/orders/preview", "order_preview.json")
	m := buyLimit("1")
	m.CurrentAsk, m.CurrentBid = Price("181"), Price("180")
	if _, err := c.PreviewCombo(context.Background(), "A", Bracket(m, sellLimit("9"), nil)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(f.gotRaw), `"current_ask"`) {
		t.Error("preview must forward the caller's quotes, as PreviewOrder does")
	}
}

func TestComboRejectsDuplicateClientIDs(t *testing.T) {
	a, b := buyLimit("1"), sellLimit("999")
	a.ClientOrderID, b.ClientOrderID = "same-id-000001", "same-id-000001"
	if err := Bracket(a, b, nil).prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("duplicate ids within a group must be rejected")
	}
}

func TestPlaceComboRequestAndReceipt(t *testing.T) {
	c, f := newBodyClient(t, "/trading/orders/place", "combo_place.json")
	combo := Bracket(buyLimit("1.00"), sellLimit("999"), sellStop("0.50"))
	got, err := c.PlaceCombo(context.Background(), "A", combo)
	if err != nil {
		t.Fatal(err)
	}
	if f.gotBody["client_combo_order_id"] != combo.ClientComboOrderID || combo.ClientComboOrderID == "" {
		t.Errorf("client_combo_order_id = %v", f.gotBody["client_combo_order_id"])
	}
	orders := f.gotBody["new_orders"].([]any)
	if len(orders) != 3 {
		t.Fatalf("sent %d orders", len(orders))
	}
	if orders[0].(map[string]any)["combo_type"] != "MASTER" || orders[2].(map[string]any)["order_type"] != "STOP_LOSS" {
		t.Errorf("orders = %v", orders)
	}
	if got.ComboOrderID == "" || got.ClientComboOrderID == "" {
		t.Errorf("receipt = %+v", got)
	}
	if got.OrderID != "" {
		t.Error("a group receipt carries no single order id")
	}
}

func TestPreviewCombo(t *testing.T) {
	c, _ := newBodyClient(t, "/trading/orders/preview", "order_preview.json")
	if _, err := c.PreviewCombo(context.Background(), "A", Bracket(buyLimit("1"), sellLimit("9"), nil)); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PreviewCombo(context.Background(), "A", OCO(buyLimit("1"))); !errors.Is(err, ErrInvalidOrder) {
		t.Error("invalid combo must be rejected locally")
	}
}

func TestComboSubOrderLookup(t *testing.T) {
	c, _ := newTestClient(t, "/trading/orders/get", "combo_get_tp.json")
	g, err := c.Order(context.Background(), "A", "clientordertp0000000001")
	if err != nil {
		t.Fatal(err)
	}
	if g.ComboType != RoleStopProfit || g.ComboOrderID == "" || len(g.Orders) != 1 {
		t.Errorf("group = %+v", g)
	}
	if len(g.Orders[0].Fees) == 0 || !g.Orders[0].Fees[0].Actual.Valid {
		t.Errorf("fees = %+v", g.Orders[0].Fees)
	}
}

func TestCancelComboAttemptsEveryOrderAndToleratesGoneOnes(t *testing.T) {
	var cancelled []string
	var fail map[string]string
	c := newClientFor(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ID string `json:"client_order_id"`
		}
		_ = decodeJSON(r, &body)
		cancelled = append(cancelled, body.ID)
		if code, ok := fail[body.ID]; ok {
			w.WriteHeader(http.StatusExpectationFailed)
			_, _ = w.Write([]byte(`{"error_code":"` + code + `","message":"x"}`))
			return
		}
		_, _ = w.Write(loadFixture(t, "order_place.json"))
	}))
	c.doer.DecodeError = func(r transportResponse) error {
		var p struct {
			Code string `json:"error_code"`
		}
		_ = json.Unmarshal(r.Body, &p)
		return &codedError{code: p.Code}
	}

	combo := Bracket(buyLimit("1"), sellLimit("9"), sellStop("0.5"))
	_ = combo.prepare()
	master, tp, sl := combo.Orders[0].ClientOrderID, combo.Orders[1].ClientOrderID, combo.Orders[2].ClientOrderID

	// Master filled and cannot be cancelled; the exits are live and must be.
	fail = map[string]string{master: "OPENAPI_ORDER_CAN_NOT_BE_CANCEL"}
	if err := c.CancelCombo(context.Background(), "A", combo); err != nil {
		t.Fatalf("a filled master is not a failure: %v", err)
	}
	if len(cancelled) != 3 {
		t.Errorf("cancelled %v; every order must be attempted", cancelled)
	}

	// Siblings already cancelled along with the first one.
	cancelled = nil
	fail = map[string]string{tp: "OPENAPI_ORDER_NOT_FOUND", sl: "OPENAPI_ORDER_NOT_FOUND"}
	if err := c.CancelCombo(context.Background(), "A", combo); err != nil {
		t.Fatalf("already-gone siblings are not failures: %v", err)
	}

	// A real failure is reported, after the others were still attempted.
	cancelled = nil
	fail = map[string]string{tp: "OPENAPI_SYSTEM_ERROR"}
	err := c.CancelCombo(context.Background(), "A", combo)
	if err == nil || !strings.Contains(err.Error(), tp) {
		t.Errorf("expected the failing order in the error, got %v", err)
	}
	if len(cancelled) != 3 {
		t.Errorf("a failure must not stop the remaining cancels: %v", cancelled)
	}

	if err := c.CancelCombo(context.Background(), "A", nil); !errors.Is(err, ErrInvalidOrder) {
		t.Error("nil combo must be rejected")
	}
}

func TestMultiLegValidation(t *testing.T) {
	call := func(side Side, strike string) OrderLeg {
		return OrderLeg{Symbol: "AAPL", Side: side, OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price(strike)}
	}
	vertical := func() *Order {
		return &Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: Price("0.50"),
			OptionStrategy: StrategyVertical, Legs: []OrderLeg{call(Buy, "240"), call(Sell, "250")}}
	}
	if err := vertical().prepare(); err != nil {
		t.Fatalf("vertical rejected: %v", err)
	}

	oneLeg := vertical()
	oneLeg.Legs = oneLeg.Legs[:1]
	if err := oneLeg.prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("a one-leg vertical must be rejected locally; the server does not check at preview")
	}

	noStrategy := vertical()
	noStrategy.OptionStrategy = ""
	if err := noStrategy.prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("two legs without a strategy must be rejected")
	}

	covered := &Order{Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1"), OptionStrategy: StrategyCoveredStock,
		Legs: []OrderLeg{{Symbol: "AAPL", Side: Buy, Quantity: Price("100"), InstrumentType: InstrumentEquity}, call(Sell, "260")}}
	if err := covered.prepare(); err != nil {
		t.Fatalf("covered stock rejected: %v", err)
	}
	if covered.Legs[0].InstrumentType != InstrumentEquity || covered.Legs[0].StrikePrice.Valid {
		t.Errorf("stock leg mangled: %+v", covered.Legs[0])
	}

	singleTwoLegs := vertical()
	singleTwoLegs.OptionStrategy = StrategySingle
	if err := singleTwoLegs.prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("SINGLE with two legs must be rejected")
	}

	unsizedStock := &Order{Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1"), OptionStrategy: StrategyCoveredStock,
		Legs: []OrderLeg{{Symbol: "AAPL", Side: Buy, InstrumentType: InstrumentEquity}, call(Sell, "260")}}
	if err := unsizedStock.prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("a stock leg without a share quantity must be rejected, not defaulted to the contract count")
	}

	badLeg := vertical()
	badLeg.Legs[0].InstrumentType = InstrumentFutures
	if err := badLeg.prepare(); !errors.Is(err, ErrInvalidOrder) {
		t.Error("legs must be OPTION or EQUITY")
	}
}

func TestMultiLegPreviewSendsAllLegs(t *testing.T) {
	c, f := newBodyClient(t, "/trading/orders/preview", "order_preview_vertical.json")
	o := &Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: Price("0.50"),
		OptionStrategy: StrategyVertical, Legs: []OrderLeg{
			{Symbol: "AAPL", Side: Buy, OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("240")},
			{Symbol: "AAPL", Side: Sell, OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("250")}}}
	p, err := c.PreviewOrder(context.Background(), "A", o)
	if err != nil {
		t.Fatal(err)
	}
	sent := firstOrder(t, f.gotBody, "new_orders")
	if sent["option_strategy"] != "VERTICAL" || len(sent["legs"].([]any)) != 2 {
		t.Errorf("sent = %v", sent)
	}
	if !p.EstimatedCost.Equal(decimal.NewFromInt(50)) {
		t.Errorf("cost = %v", p.EstimatedCost)
	}
	if strings.Contains(string(f.gotRaw), `"support_trading_session"`) {
		t.Error("option orders should not carry the equity session default")
	}
}

func TestComboErrorsPropagate(t *testing.T) {
	c := newClientFor(t, &bodyFixture{fixture: fixture{t: t, body: loadFixture(t, "combo_rejected_oco.json"), status: http.StatusExpectationFailed}})
	ctx := context.Background()
	var te *testError
	if _, err := c.PlaceCombo(ctx, "A", Bracket(buyLimit("1"), sellLimit("9"), nil)); !errors.As(err, &te) {
		t.Errorf("PlaceCombo: got %v", err)
	}
	if _, err := c.PreviewCombo(ctx, "A", Bracket(buyLimit("1"), sellLimit("9"), nil)); !errors.As(err, &te) {
		t.Errorf("PreviewCombo: got %v", err)
	}
	if _, err := c.PlaceCombo(ctx, "A", OCO(buyLimit("1"))); !errors.Is(err, ErrInvalidOrder) {
		t.Errorf("PlaceCombo must validate locally first: %v", err)
	}
}

func TestPlaceComboClassifiesDuplicates(t *testing.T) {
	c := newClientFor(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusExpectationFailed) }))
	c.doer.DecodeError = func(transportResponse) error { return &codedError{code: duplicateOrderCode} }
	_, err := c.PlaceCombo(context.Background(), "A", Bracket(buyLimit("1"), sellLimit("9"), nil))
	if !errors.Is(err, ErrDuplicateOrder) {
		t.Errorf("got %v, want ErrDuplicateOrder", err)
	}
}

func TestAlreadyDoneIgnoresUncodedErrors(t *testing.T) {
	if alreadyDone(errors.New("network down")) {
		t.Error("an error without a code cannot mean the order was already gone")
	}
	if alreadyDone(&codedError{code: "OPENAPI_SYSTEM_ERROR"}) {
		t.Error("only the not-found and cannot-cancel codes mean already done")
	}
	if !alreadyDone(&codedError{code: "OPENAPI_ORDER_NOT_FOUND"}) {
		t.Error("not-found means already done")
	}
}

func TestOppositeSides(t *testing.T) {
	for _, tc := range []struct {
		a, b Side
		want bool
	}{
		{Buy, Sell, true}, {Sell, Buy, true}, {Short, Buy, true},
		{Buy, Buy, false}, {Sell, Sell, false}, {Buy, Short, false}, {"", Buy, false},
	} {
		if got := opposite(tc.a, tc.b); got != tc.want {
			t.Errorf("opposite(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestCancelComboSkipsNilAndReportsNothingForNoOrders(t *testing.T) {
	c := newClientFor(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(loadFixture(t, "order_place.json")) }))
	combo := &Combo{Orders: []*Order{nil, buyLimit("1")}}
	combo.Orders[1].ClientOrderID = "clientorder0000000001"
	if err := c.CancelCombo(context.Background(), "A", combo); err != nil {
		t.Errorf("nil entries should be skipped, got %v", err)
	}
}
