package trade

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// bodyFixture serves a fixture and captures the JSON body it received.
type bodyFixture struct {
	fixture
	gotBody map[string]any
	gotRaw  []byte
}

func (f *bodyFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.gotRaw, _ = io.ReadAll(r.Body)
	_ = json.Unmarshal(f.gotRaw, &f.gotBody)
	f.fixture.ServeHTTP(w, r)
}

func newBodyClient(t *testing.T, wantPath, file string) (*Client, *bodyFixture) {
	t.Helper()
	f := &bodyFixture{fixture: fixture{t: t, wantPath: wantPath, body: loadFixture(t, file)}}
	return newClientFor(t, f), f
}

func firstOrder(t *testing.T, body map[string]any, key string) map[string]any {
	t.Helper()
	list, ok := body[key].([]any)
	if !ok || len(list) != 1 {
		t.Fatalf("%s = %v, want one order", key, body[key])
	}
	return list[0].(map[string]any)
}

func TestPriceHelper(t *testing.T) {
	p := Price("180.50")
	if !p.Valid || !p.Decimal.Equal(decimal.RequireFromString("180.5")) {
		t.Errorf("Price = %+v", p)
	}
	defer func() {
		if recover() == nil {
			t.Error("Price should panic on a malformed literal")
		}
	}()
	Price("not a number")
}

func TestPrepareFillsEquityDefaults(t *testing.T) {
	o := Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("10"), LimitPrice: Price("180")}
	if err := o.prepare(); err != nil {
		t.Fatal(err)
	}
	if o.ClientOrderID == "" || len(o.ClientOrderID) < clientOrderIDMin || len(o.ClientOrderID) > clientOrderIDMax {
		t.Errorf("ClientOrderID = %q", o.ClientOrderID)
	}
	want := Order{InstrumentType: InstrumentEquity, ComboType: Normal, EntrustType: ByQuantity,
		Market: "US", TradingSession: SessionCore, TimeInForce: Day}
	if o.InstrumentType != want.InstrumentType || o.ComboType != want.ComboType || o.EntrustType != want.EntrustType ||
		o.Market != want.Market || o.TradingSession != want.TradingSession || o.TimeInForce != want.TimeInForce {
		t.Errorf("defaults = %+v", o)
	}
}

func TestPrepareGeneratesUniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		id := newClientOrderID()
		if seen[id] {
			t.Fatal("duplicate client order id")
		}
		seen[id] = true
	}
}

func TestPreparePreservesCallerID(t *testing.T) {
	o := Order{ClientOrderID: "my-own-id-0001", Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1")}
	if err := o.prepare(); err != nil {
		t.Fatal(err)
	}
	if o.ClientOrderID != "my-own-id-0001" {
		t.Errorf("caller's id was replaced: %q", o.ClientOrderID)
	}
}

func TestPrepareOptionDefaults(t *testing.T) {
	leg, err := LegFromSymbol("AAPL261218C00240000")
	if err != nil {
		t.Fatal(err)
	}
	o := Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: Price("5.50"),
		PositionIntent: BuyToOpen, Legs: []OrderLeg{leg}}
	if err := o.prepare(); err != nil {
		t.Fatal(err)
	}
	if o.InstrumentType != InstrumentOption {
		t.Errorf("InstrumentType = %q, want OPTION when legs are set", o.InstrumentType)
	}
	if o.OptionStrategy != StrategySingle {
		t.Errorf("OptionStrategy = %q, want SINGLE for one leg", o.OptionStrategy)
	}
	if o.TradingSession != "" {
		t.Errorf("TradingSession = %q; options should not receive the equity default", o.TradingSession)
	}
	l := o.Legs[0]
	if l.Side != Buy || !l.Quantity.Valid || l.Market != "US" || l.InstrumentType != InstrumentOption {
		t.Errorf("leg defaults = %+v", l)
	}
}

func TestPrepareValidation(t *testing.T) {
	q := Price("1")
	cases := map[string]Order{
		"no symbol":             {Side: Buy, Type: Market, Quantity: q},
		"no side":               {Symbol: "AAPL", Type: Market, Quantity: q},
		"no type":               {Symbol: "AAPL", Side: Buy, Quantity: q},
		"limit without price":   {Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: q},
		"stop without stop":     {Symbol: "AAPL", Side: Buy, Type: StopLoss, Quantity: q},
		"stop limit half":       {Symbol: "AAPL", Side: Buy, Type: StopLossLimit, Quantity: q, LimitPrice: q},
		"trailing incomplete":   {Symbol: "AAPL", Side: Buy, Type: TrailingStopLoss, Quantity: q, TrailingType: TrailByAmount},
		"gtd without date":      {Symbol: "AAPL", Side: Buy, Type: Market, Quantity: q, TimeInForce: GTD},
		"qty missing":           {Symbol: "AAPL", Side: Buy, Type: Market},
		"amount missing":        {Symbol: "AAPL", Side: Buy, Type: Market, EntrustType: ByAmount},
		"short client id":       {ClientOrderID: "short", Symbol: "AAPL", Side: Buy, Type: Market, Quantity: q},
		"long client id":        {ClientOrderID: strings.Repeat("x", 41), Symbol: "AAPL", Side: Buy, Type: Market, Quantity: q},
		"option without legs":   {Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: q, LimitPrice: q, InstrumentType: InstrumentOption},
		"option leg incomplete": {Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: q, LimitPrice: q, Legs: []OrderLeg{{Symbol: "AAPL", OptionType: Call}}},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			err := o.prepare()
			if !errors.Is(err, ErrInvalidOrder) {
				t.Errorf("got %v, want ErrInvalidOrder", err)
			}
		})
	}
}

func TestPrepareAcceptsZeroPrice(t *testing.T) {
	// A limit price of zero is unusual but expressible: NullDecimal set to
	// zero is Valid and must pass validation.
	o := Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: decimal.NewNullDecimal(decimal.Zero)}
	if err := o.prepare(); err != nil {
		t.Errorf("zero limit price rejected: %v", err)
	}
}

func TestLegFromSymbol(t *testing.T) {
	for sym, want := range map[string]OrderLeg{
		"AAPL261218C00240000":  {Symbol: "AAPL", ExpireDate: "2026-12-18", OptionType: Call, StrikePrice: Price("240")},
		"SPXW250620P04200000":  {Symbol: "SPXW", ExpireDate: "2025-06-20", OptionType: Put, StrikePrice: Price("4200")},
		"BRK.B260116C00450500": {Symbol: "BRK.B", ExpireDate: "2026-01-16", OptionType: Call, StrikePrice: Price("450.5")},
	} {
		got, err := LegFromSymbol(sym)
		if err != nil {
			t.Errorf("%s: %v", sym, err)
			continue
		}
		if got.Symbol != want.Symbol || got.ExpireDate != want.ExpireDate || got.OptionType != want.OptionType ||
			!got.StrikePrice.Decimal.Equal(want.StrikePrice.Decimal) {
			t.Errorf("%s: got %+v", sym, got)
		}
	}
	for _, bad := range []string{"AAPL", "AAPL261218X00240000", "aapl261218C00240000", "AAPL261318C00240000", ""} {
		if _, err := LegFromSymbol(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

func TestOrderJSONOmitsUnsetOptionals(t *testing.T) {
	o := Order{Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("10")}
	if err := o.prepare(); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(o)
	s := string(b)
	for _, absent := range []string{"limit_price", "stop_price", "legs", "trailing", "algo_", "expire_date", "total_cash_amount"} {
		if strings.Contains(s, absent) {
			t.Errorf("unset %s was serialised: %s", absent, s)
		}
	}
	for _, present := range []string{`"quantity":"10"`, `"market":"US"`, `"combo_type":"NORMAL"`, `"support_trading_session":"CORE"`} {
		if !strings.Contains(s, present) {
			t.Errorf("missing %s in %s", present, s)
		}
	}
}

func TestPlaceOrderRequestAndReceipt(t *testing.T) {
	c, f := newBodyClient(t, "/trading/orders/place", "order_place.json")
	o := &Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: Price("1.00")}
	got, err := c.PlaceOrder(context.Background(), "ACCT1", o)
	if err != nil {
		t.Fatal(err)
	}
	if f.gotBody["account_id"] != "ACCT1" {
		t.Errorf("account_id = %v", f.gotBody["account_id"])
	}
	sent := firstOrder(t, f.gotBody, "new_orders")
	if sent["client_order_id"] != o.ClientOrderID {
		t.Error("the generated client order id was not written back to the caller's order")
	}
	if sent["limit_price"] != "1" || sent["quantity"] != "1" || sent["order_type"] != "LIMIT" {
		t.Errorf("sent = %v", sent)
	}
	if got.OrderID == "" || got.ClientOrderID == "" {
		t.Errorf("receipt = %+v", got)
	}
}

func TestPlaceOrderRejectsInvalidLocally(t *testing.T) {
	c, f := newBodyClient(t, "", "order_place.json")
	_, err := c.PlaceOrder(context.Background(), "A", &Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1")})
	if !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("got %v", err)
	}
	if f.gotRaw != nil {
		t.Error("an invalid order must not reach the network")
	}
}

func TestPreviewOrder(t *testing.T) {
	c, f := newBodyClient(t, "/trading/orders/preview", "order_preview_option.json")
	leg, _ := LegFromSymbol("AAPL261218C00240000")
	o := &Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: Price("5.50"),
		PositionIntent: BuyToOpen, Legs: []OrderLeg{leg}}
	got, err := c.PreviewOrder(context.Background(), "A", o)
	if err != nil {
		t.Fatal(err)
	}
	sent := firstOrder(t, f.gotBody, "new_orders")
	legs := sent["legs"].([]any)
	l := legs[0].(map[string]any)
	if l["strike_price"] != "240" || l["option_expire_date"] != "2026-12-18" || l["option_type"] != "CALL" || l["side"] != "BUY" {
		t.Errorf("leg = %v", l)
	}
	if !got.EstimatedCost.Equal(decimal.RequireFromString("550")) || got.Currency != "USD" {
		t.Errorf("preview = %+v", got)
	}
}

func TestReplaceOrder(t *testing.T) {
	c, f := newBodyClient(t, "/trading/orders/replace", "order_place.json")
	_, err := c.ReplaceOrder(context.Background(), "A", OrderModification{ClientOrderID: "clientorder0001", LimitPrice: Price("1.05")})
	if err != nil {
		t.Fatal(err)
	}
	sent := firstOrder(t, f.gotBody, "modify_orders")
	if sent["client_order_id"] != "clientorder0001" || sent["limit_price"] != "1.05" {
		t.Errorf("sent = %v", sent)
	}
	if _, has := sent["quantity"]; has {
		t.Error("unset quantity must be omitted, not sent as null")
	}
	if _, err := c.ReplaceOrder(context.Background(), "A", OrderModification{}); !errors.Is(err, ErrInvalidOrder) {
		t.Error("missing client order id should be rejected locally")
	}
}

func TestCancelOrder(t *testing.T) {
	c, f := newBodyClient(t, "/trading/orders/cancel", "order_place.json")
	if _, err := c.CancelOrder(context.Background(), "A", "clientorder0001"); err != nil {
		t.Fatal(err)
	}
	if f.gotBody["client_order_id"] != "clientorder0001" || f.gotBody["account_id"] != "A" {
		t.Errorf("body = %v", f.gotBody)
	}
	if _, err := c.CancelOrder(context.Background(), "A", ""); !errors.Is(err, ErrInvalidOrder) {
		t.Error("empty client order id should be rejected locally")
	}
}

func TestOrderDetail(t *testing.T) {
	c, f := newTestClient(t, "/trading/orders/get", "order_get_cancelled.json")
	got, err := c.Order(context.Background(), "A", "clientorder0001")
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("client_order_id") != "clientorder0001" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if got.ComboType != Normal || len(got.Orders) != 1 {
		t.Fatalf("group = %+v", got)
	}
	o := got.Orders[0]
	if o.Status != StatusCancelled || o.Type != Limit || o.TradingSession != SessionCore {
		t.Errorf("order = %+v", o)
	}
	if !o.LimitPrice.Valid || !o.LimitPrice.Decimal.Equal(decimal.RequireFromString("1.05")) {
		t.Errorf("LimitPrice = %v", o.LimitPrice)
	}
	if o.FilledPrice.Valid {
		t.Error("an unfilled order must report no fill price")
	}
	if o.PlaceTime.IsZero() || !o.FilledTime.IsZero() {
		t.Errorf("times: place=%v filled=%v", o.PlaceTime, o.FilledTime)
	}
	if o.Fees == nil || len(o.Fees) != 0 {
		t.Errorf("Fees = %v, want empty list", o.Fees)
	}
}

func TestOpenOrdersAndHistory(t *testing.T) {
	c, f := newTestClient(t, "/trading/orders/open-orders/list", "order_open.json")
	got, err := c.OpenOrders(context.Background(), OrdersRequest{AccountID: "A", PageSize: 10, LastClientOrderID: "prev"})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("page_size") != "10" || f.gotQuery.Get("last_client_order_id") != "prev" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if _, has := f.gotQuery["start_date"]; has {
		t.Error("start_date applies to history only")
	}
	if len(got) != 1 || got[0].Orders[0].Status != StatusSubmitted {
		t.Errorf("open = %+v", got)
	}

	c2, f2 := newTestClient(t, "/trading/orders/historical-orders/list", "order_history.json")
	hist, err := c2.OrderHistory(context.Background(), OrdersRequest{AccountID: "A", StartDate: "2026-08-01", EndDate: "2026-08-31"})
	if err != nil {
		t.Fatal(err)
	}
	if f2.gotQuery.Get("start_date") != "2026-08-01" || f2.gotQuery.Get("end_date") != "2026-08-31" {
		t.Errorf("query = %v", f2.gotQuery)
	}
	if len(hist) != 1 {
		t.Errorf("history = %+v", hist)
	}
}

func TestPlaceOrdersBatchConstraints(t *testing.T) {
	c, f := newBodyClient(t, "/trading/orders/batch-place", "order_place.json")
	ok := &Order{Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: Price("1")}
	for name, bad := range map[string]*Order{
		"gtc":    {Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1"), TimeInForce: GTC},
		"stop":   {Symbol: "AAPL", Side: Buy, Type: StopLoss, Quantity: Price("1"), StopPrice: Price("1")},
		"short":  {Symbol: "AAPL", Side: Short, Type: Market, Quantity: Price("1")},
		"option": {Symbol: "AAPL", Side: Buy, Type: Limit, Quantity: Price("1"), LimitPrice: Price("1"), Legs: []OrderLeg{{Symbol: "AAPL", OptionType: Call, ExpireDate: "2026-12-18", StrikePrice: Price("240")}}},
	} {
		if _, err := c.PlaceOrders(context.Background(), "A", []*Order{ok, bad}); !errors.Is(err, ErrInvalidOrder) {
			t.Errorf("%s: got %v, want ErrInvalidOrder", name, err)
		}
	}
	if _, err := c.PlaceOrders(context.Background(), "A", nil); !errors.Is(err, ErrInvalidOrder) {
		t.Error("empty batch should be rejected")
	}
	if f.gotRaw != nil {
		t.Error("no invalid batch should reach the network")
	}
}

func TestPlaceOrdersSendsBatch(t *testing.T) {
	c, f := newBodyClient(t, "/trading/orders/batch-place", "order_place.json")
	a := &Order{Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1")}
	b := &Order{Symbol: "MSFT", Side: Sell, Type: Limit, Quantity: Price("2"), LimitPrice: Price("400")}
	// The fixture is a single-order receipt; decoding into BatchResult just
	// leaves counts at zero, which is fine for asserting on the request.
	if _, err := c.PlaceOrders(context.Background(), "A", []*Order{a, b}); err != nil {
		t.Fatal(err)
	}
	list := f.gotBody["batch_orders"].([]any)
	if len(list) != 2 || list[1].(map[string]any)["symbol"] != "MSFT" {
		t.Errorf("batch = %v", list)
	}
}

func TestOrderErrorsPropagate(t *testing.T) {
	c := newClientFor(t, &bodyFixture{fixture: fixture{t: t, body: loadFixture(t, "order_preview_badleg.json"), status: http.StatusExpectationFailed}})
	ctx := context.Background()
	valid := func() *Order { return &Order{Symbol: "AAPL", Side: Buy, Type: Market, Quantity: Price("1")} }
	calls := map[string]func() error{
		"PlaceOrder":   func() error { _, err := c.PlaceOrder(ctx, "A", valid()); return err },
		"PreviewOrder": func() error { _, err := c.PreviewOrder(ctx, "A", valid()); return err },
		"ReplaceOrder": func() error {
			_, err := c.ReplaceOrder(ctx, "A", OrderModification{ClientOrderID: "clientorder0001"})
			return err
		},
		"CancelOrder":  func() error { _, err := c.CancelOrder(ctx, "A", "clientorder0001"); return err },
		"Order":        func() error { _, err := c.Order(ctx, "A", "clientorder0001"); return err },
		"OpenOrders":   func() error { _, err := c.OpenOrders(ctx, OrdersRequest{AccountID: "A"}); return err },
		"OrderHistory": func() error { _, err := c.OrderHistory(ctx, OrdersRequest{AccountID: "A"}); return err },
		"PlaceOrders":  func() error { _, err := c.PlaceOrders(ctx, "A", []*Order{valid()}); return err },
	}
	for name, call := range calls {
		var te *testError
		if err := call(); !errors.As(err, &te) || te.status != http.StatusExpectationFailed {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestPreviewOrderRejectsInvalidLocally(t *testing.T) {
	c, f := newBodyClient(t, "", "order_preview.json")
	_, err := c.PreviewOrder(context.Background(), "A", &Order{Symbol: "AAPL", Side: Buy, Type: StopLoss, Quantity: Price("1")})
	if !errors.Is(err, ErrInvalidOrder) {
		t.Fatalf("got %v", err)
	}
	if f.gotRaw != nil {
		t.Error("an invalid preview must not reach the network")
	}
}
