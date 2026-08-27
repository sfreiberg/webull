package trade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/sfreiberg/webull/internal/signing"
	"github.com/sfreiberg/webull/internal/transport"
)

// fixture serves testdata/<name>.json for one expected path and records the
// query it received, so each test can assert on both the request and the
// decoded result.
type fixture struct {
	t        *testing.T
	wantPath string
	body     []byte
	status   int
	gotQuery url.Values
	gotPath  string
}

// ServeHTTP runs on the server goroutine, so it must not call FailNow; it
// records what it saw and uses Errorf. The body was read on the test
// goroutine by loadFixture.
func (f *fixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.gotPath = r.URL.Path
	f.gotQuery = r.URL.Query()
	if f.wantPath != "" && r.URL.Path != f.wantPath {
		f.t.Errorf("path = %q, want %q", r.URL.Path, f.wantPath)
	}
	if f.status != 0 {
		w.WriteHeader(f.status)
	}
	_, _ = w.Write(f.body)
}

func loadFixture(t *testing.T, file string) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	return body
}

type testError struct{ status int }

func (e *testError) Error() string { return fmt.Sprintf("status %d", e.status) }

// newClientFor wires a Client to any handler with the test transport: no
// retries, no sleeps, and errors decoded into *testError.
func newClientFor(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	return New(&transport.Doer{
		HTTPClient:  srv.Client(),
		Signer:      signing.New("k", "s"),
		DecodeError: func(r transport.Response) error { return &testError{status: r.StatusCode} },
		Retry:       transport.RetryPolicy{MaxAttempts: 1},
	}, strings.TrimPrefix(srv.URL, "https://"))
}

func newTestClient(t *testing.T, wantPath, file string) (*Client, *fixture) {
	t.Helper()
	f := &fixture{t: t, wantPath: wantPath, body: loadFixture(t, file)}
	return newClientFor(t, f), f
}

func mustDecimal(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	return decimal.RequireFromString(s)
}

func TestAccounts(t *testing.T) {
	c, _ := newTestClient(t, "/trading/accounts/list", "accounts.json")
	got, err := c.Accounts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d accounts, want 2", len(got))
	}
	if got[0].AccountType != AccountTypeMargin || got[0].AccountClass != AccountClassIndividualMargin {
		t.Errorf("account[0] = %+v", got[0])
	}
}

func TestBalanceDecodesDecimalsAndAbsentFields(t *testing.T) {
	c, f := newTestClient(t, "/trading/assets/balances/get", "balances.json")
	got, err := c.Balance(context.Background(), "ACCT1")
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("account_id") != "ACCT1" {
		t.Errorf("account_id query = %q", f.gotQuery.Get("account_id"))
	}

	if !got.TotalCashBalance.Equal(mustDecimal(t, "1000000.00")) {
		t.Errorf("TotalCashBalance = %v", got.TotalCashBalance)
	}
	if got.DayTradesLeft != "UNLIMITED" {
		t.Errorf("DayTradesLeft = %q", got.DayTradesLeft)
	}
	// The fixture omits every margin figure. Absent must be reported as
	// absent, not as zero.
	if got.UsedMargin.Valid {
		t.Error("UsedMargin should be invalid when the field is absent")
	}
	if got.OpenMarginCalls == nil || len(got.OpenMarginCalls) != 0 {
		t.Errorf("OpenMarginCalls = %v, want empty non-nil list", got.OpenMarginCalls)
	}
	if len(got.CurrencyAssets) != 1 || got.CurrencyAssets[0].Currency != "USD" {
		t.Errorf("CurrencyAssets = %+v", got.CurrencyAssets)
	}
	if !got.CurrencyAssets[0].OptionBuyingPower.Valid {
		t.Error("OptionBuyingPower is present in the fixture and should be valid")
	}
	if got.CurrencyAssets[0].SettledCash.Valid {
		t.Error("SettledCash is absent from the fixture and should be invalid")
	}
}

func TestPositionsEmpty(t *testing.T) {
	c, _ := newTestClient(t, "/trading/assets/positions/list", "positions.json")
	got, err := c.Positions(context.Background(), "ACCT1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %d positions from an empty fixture", len(got))
	}
}

func TestCashActivitiesRequestEncoding(t *testing.T) {
	c, f := newTestClient(t, "/trading/activities/cash-activities/list", "cash_activities.json")
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 27, 23, 59, 59, 500_000_000, time.FixedZone("EDT", -4*3600))

	got, err := c.CashActivities(context.Background(), CashActivitiesRequest{
		AccountID:      "ACCT1",
		ActivityTypes:  []ActivityType{ActivityDeposit, ActivityTrade},
		StartTime:      start,
		EndTime:        end,
		LastActivityID: "prev",
		PageSize:       50,
	})
	if err != nil {
		t.Fatal(err)
	}

	for k, want := range map[string]string{
		"account_id":       "ACCT1",
		"activity_types":   "DEPOSIT,TRADE",
		"start_time":       "2026-08-01T00:00:00.000Z",
		"end_time":         "2026-08-28T03:59:59.500Z", // converted to UTC
		"last_activity_id": "prev",
		"page_size":        "50",
	} {
		if got := f.gotQuery.Get(k); got != want {
			t.Errorf("query %s = %q, want %q", k, got, want)
		}
	}

	if len(got) != 1 {
		t.Fatalf("got %d activities", len(got))
	}
	a := got[0]
	if a.ActivityType != ActivityDeposit || a.ActivitySubType != SubTypeWire {
		t.Errorf("activity = %+v", a)
	}
	if !a.NetAmount.Equal(mustDecimal(t, "1000000")) {
		t.Errorf("NetAmount = %v", a.NetAmount)
	}
	if a.BizTime.IsZero() || a.BizTime.Year() != 2026 {
		t.Errorf("BizTime = %v; the millisecond ISO-8601 form must parse", a.BizTime)
	}
}

func TestCashActivitiesOmitsUnsetOptionals(t *testing.T) {
	c, f := newTestClient(t, "", "cash_activities.json")
	if _, err := c.CashActivities(context.Background(), CashActivitiesRequest{AccountID: "A"}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"activity_types", "start_time", "end_time", "last_activity_id", "page_size"} {
		if _, present := f.gotQuery[k]; present {
			t.Errorf("unset optional %s was sent as %q", k, f.gotQuery.Get(k))
		}
	}
}

func TestStockProfiles(t *testing.T) {
	c, f := newTestClient(t, "/trading/instruments/stocks/profiles/list", "stocks.json")
	got, err := c.StockProfiles(context.Background(), StockProfilesRequest{Symbols: []string{"AAPL", "SPY"}})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("category") != "US_STOCK" || f.gotQuery.Get("symbols") != "AAPL,SPY" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if got.PaginationKey != "" {
		t.Errorf("a symbol lookup should have no pagination key, got %q", got.PaginationKey)
	}
	aapl := got.Profiles[0]
	if aapl.Symbol != "AAPL" || aapl.InstrumentID != "913256135" || !aapl.Shortable {
		t.Errorf("AAPL = %+v", aapl)
	}
	if !aapl.MarginRequirementLong.Valid || !aapl.MarginRequirementLong.Decimal.Equal(mustDecimal(t, "0.5")) {
		t.Errorf("MarginRequirementLong = %v", aapl.MarginRequirementLong)
	}
}

func TestStockProfilesPagination(t *testing.T) {
	c, f := newTestClient(t, "", "stocks_paged.json")
	got, err := c.StockProfiles(context.Background(), StockProfilesRequest{SubCategory: ETF, PaginationKey: "cursor-1"})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("pagination_key") != "cursor-1" || f.gotQuery.Get("sub_category") != "ETF" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if got.PaginationKey == "" {
		t.Error("expected a pagination key for a browse with more pages")
	}
}

func TestCryptoProfiles(t *testing.T) {
	c, f := newTestClient(t, "/trading/instruments/crypto/profiles/list", "crypto.json")
	got, err := c.CryptoProfiles(context.Background(), CryptoProfilesRequest{Symbols: []string{"BTCUSD", "ETHUSD"}, Status: Tradable})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("category") != "US_CRYPTO" || f.gotQuery.Get("status") != "OC" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if got.Profiles[0].Symbol != "BTCUSD" || !got.Profiles[0].PriceStep.Decimal.Equal(mustDecimal(t, "0.01")) {
		t.Errorf("BTCUSD = %+v", got.Profiles[0])
	}
}

func TestOptionContractsRequestEncoding(t *testing.T) {
	c, f := newTestClient(t, "/trading/instruments/options/contracts/list", "options.json")
	gte, lte := mustDecimal(t, "150"), mustDecimal(t, "250.5")
	yes := true
	got, err := c.OptionContracts(context.Background(), OptionContractsRequest{
		UnderlyingSymbols: []string{"AAPL"},
		OptionType:        Call,
		Style:             American,
		StrikePriceGTE:    &gte,
		StrikePriceLTE:    &lte,
		PPInd:             &yes,
		ShowDeliverables:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"category": "US_OPTION", "underlying_symbols": "AAPL", "option_type": "CALL",
		"style": "AMERICAN", "strike_price_gte": "150", "strike_price_lte": "250.5",
		"ppind": "true", "show_deliverables": "TRUE",
	} {
		if got := f.gotQuery.Get(k); got != want {
			t.Errorf("query %s = %q, want %q", k, got, want)
		}
	}
	first := got.Contracts[0]
	if first.Symbol != "AAPL261218C00240000" || first.OptionType != Call || !first.PPInd {
		t.Errorf("contract = %+v", first)
	}
	if !first.StrikePrice.Equal(mustDecimal(t, "240")) || !first.Multiplier.Equal(mustDecimal(t, "100")) {
		t.Errorf("strike/multiplier = %v / %v", first.StrikePrice, first.Multiplier)
	}
	if len(first.ListedExchanges) == 0 {
		t.Error("expected listed exchanges")
	}
	if got.PaginationKey == "" {
		t.Error("an AAPL chain spans pages; expected a pagination key")
	}
}

func TestOptionContractsOmitsUnsetOptionals(t *testing.T) {
	c, f := newTestClient(t, "", "options.json")
	if _, err := c.OptionContracts(context.Background(), OptionContractsRequest{}); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"ppind", "show_deliverables", "strike_price_gte", "option_type", "pagination_key"} {
		if _, present := f.gotQuery[k]; present {
			t.Errorf("unset optional %s was sent", k)
		}
	}
	if f.gotQuery.Get("category") != "US_OPTION" {
		t.Error("category is always sent")
	}
}

func TestFuturesProductClasses(t *testing.T) {
	c, f := newTestClient(t, "/trading/instruments/futures/product-classes/list", "futures_classes.json")
	got, err := c.FuturesProductClasses(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("category") != "US_FUTURES" {
		t.Error("category missing")
	}
	if got[0].ID != 1 || got[0].Name != "Energy" {
		t.Errorf("class[0] = %+v", got[0])
	}
}

func TestFuturesProducts(t *testing.T) {
	c, f := newTestClient(t, "/trading/instruments/futures/product-codes/list", "futures_products.json")
	got, err := c.FuturesProducts(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("product_class_id") != "3" {
		t.Errorf("product_class_id = %q", f.gotQuery.Get("product_class_id"))
	}
	if got[0].ProductClassName != "Cryptocurrencies" {
		t.Errorf("product[0] = %+v", got[0])
	}

	_, _ = c.FuturesProducts(context.Background(), 0)
	if _, present := f.gotQuery["product_class_id"]; present {
		t.Error("zero class id should be omitted, meaning all classes")
	}
}

func TestFuturesContractsDecodesIntegerUnit(t *testing.T) {
	c, f := newTestClient(t, "/trading/instruments/futures/contracts/list", "futures_contracts.json")
	got, err := c.FuturesContracts(context.Background(), FuturesContractsRequest{Code: "ES"})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("code") != "ES" || f.gotQuery.Get("category") != "US_FUTURES" {
		t.Errorf("query = %v", f.gotQuery)
	}
	es := got[0]
	if es.Symbol != "ESU6" || es.ContractType != ContractMonthly || es.Settlement != SettlementCash {
		t.Errorf("contract = %+v", es)
	}
	// Webull documents unit as a string enumeration; the wire carries an int.
	if es.Unit != 1 || es.Unit.String() != "Index points" {
		t.Errorf("Unit = %d (%q)", es.Unit, es.Unit.String())
	}
	if !es.Size.Equal(mustDecimal(t, "50")) || !es.MinTick.Equal(mustDecimal(t, "0.25")) {
		t.Errorf("size/tick = %v / %v", es.Size, es.MinTick)
	}
}

func TestFuturesUnitUnknownFallsBackToNumber(t *testing.T) {
	if got := FuturesUnit(9999).String(); got != "9999" {
		t.Errorf("unknown unit String() = %q", got)
	}
}

func TestEventCategoriesDecodesIntegerID(t *testing.T) {
	c, _ := newTestClient(t, "/trading/instruments/event-contracts/categories/list", "event_categories.json")
	got, err := c.EventCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Documented as a string; the wire carries an integer.
	if got[0].ID != 6 || got[0].Code != EventClimateWeather {
		t.Errorf("category[0] = %+v", got[0])
	}
}

func TestEventSeries(t *testing.T) {
	c, f := newTestClient(t, "/trading/instruments/event-contracts/series/list", "event_series.json")
	got, err := c.EventSeries(context.Background(), EventSeriesRequest{Category: EventEconomics})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("category") != "ECONOMICS" {
		t.Error("category missing")
	}
	if got.Series[0].Symbol != "KXFEDDECISION" || got.Series[0].Frequency != FrequencyCustom {
		t.Errorf("series[0] = %+v", got.Series[0])
	}
}

func TestEvents(t *testing.T) {
	c, f := newTestClient(t, "/trading/instruments/event-contracts/events/list", "event_events.json")
	got, err := c.Events(context.Background(), EventsRequest{SeriesSymbol: "KXRATECUTCOUNT", Status: EventActive})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("series_symbol") != "KXRATECUTCOUNT" || f.gotQuery.Get("status") != "ACTIVE" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if got[0].Symbol != "KXRATECUTCOUNT-26DEC31" || !got[0].MutuallyExclusive {
		t.Errorf("event[0] = %+v", got[0])
	}
}

func TestEventMarkets(t *testing.T) {
	c, f := newTestClient(t, "/trading/instruments/event-contracts/markets/list", "event_markets.json")
	got, err := c.EventMarkets(context.Background(), EventMarketsRequest{SeriesSymbol: "KXRATECUTCOUNT", ExpirationDateAfter: "2026-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery.Get("expiration_date_after") != "2026-01-01" {
		t.Error("expiration_date_after missing")
	}
	m := got.Markets[0]
	if m.Status != EventMarketListing || m.TradableStatus != Tradable || !m.CanCloseEarly {
		t.Errorf("market[0] = %+v", m)
	}
	if len(m.PriceRanges) != 1 || !m.PriceRanges[0].Step.Equal(mustDecimal(t, "0.001")) {
		t.Errorf("price ranges = %+v", m.PriceRanges)
	}
}

// TestErrorsPropagate drives every method through a 403 so that no call path
// swallows the decoded error.
func TestErrorsPropagate(t *testing.T) {
	c := newClientFor(t, &fixture{t: t, body: loadFixture(t, "positions.json"), status: http.StatusForbidden})
	ctx := context.Background()

	calls := map[string]func() error{
		"Accounts":              func() error { _, err := c.Accounts(ctx); return err },
		"Balance":               func() error { _, err := c.Balance(ctx, "A"); return err },
		"Positions":             func() error { _, err := c.Positions(ctx, "A"); return err },
		"CashActivities":        func() error { _, err := c.CashActivities(ctx, CashActivitiesRequest{}); return err },
		"StockProfiles":         func() error { _, err := c.StockProfiles(ctx, StockProfilesRequest{}); return err },
		"CryptoProfiles":        func() error { _, err := c.CryptoProfiles(ctx, CryptoProfilesRequest{}); return err },
		"OptionContracts":       func() error { _, err := c.OptionContracts(ctx, OptionContractsRequest{}); return err },
		"FuturesProductClasses": func() error { _, err := c.FuturesProductClasses(ctx); return err },
		"FuturesProducts":       func() error { _, err := c.FuturesProducts(ctx, 0); return err },
		"FuturesContracts":      func() error { _, err := c.FuturesContracts(ctx, FuturesContractsRequest{}); return err },
		"EventCategories":       func() error { _, err := c.EventCategories(ctx); return err },
		"EventSeries":           func() error { _, err := c.EventSeries(ctx, EventSeriesRequest{}); return err },
		"Events":                func() error { _, err := c.Events(ctx, EventsRequest{}); return err },
		"EventMarkets":          func() error { _, err := c.EventMarkets(ctx, EventMarketsRequest{}); return err },
	}
	for name, call := range calls {
		err := call()
		var te *testError
		if err == nil || !errorsAs(err, &te) || te.status != http.StatusForbidden {
			t.Errorf("%s: expected the decoded 403 to propagate, got %v", name, err)
		}
	}
}

func errorsAs(err error, target **testError) bool {
	return errors.As(err, target)
}

func TestParamsHelpers(t *testing.T) {
	p := params{}
	p.set("empty", "")
	p.set("full", "x")
	p.setList("none", nil)
	p.setList("list", []string{"a", "b"})
	p.setInt("zero", 0)
	p.setInt("n", 7)
	p.setBool("nilbool", nil)
	f := false
	p.setBool("f", &f)

	v := url.Values(p)
	if _, ok := v["empty"]; ok {
		t.Error("empty string should be omitted")
	}
	if _, ok := v["none"]; ok {
		t.Error("empty list should be omitted")
	}
	if _, ok := v["zero"]; ok {
		t.Error("zero int should be omitted")
	}
	if _, ok := v["nilbool"]; ok {
		t.Error("nil bool should be omitted")
	}
	if v.Get("full") != "x" || v.Get("list") != "a,b" || v.Get("n") != "7" || v.Get("f") != "false" {
		t.Errorf("values = %v", v)
	}
}

// decodeJSON reads a request body into v.
func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
