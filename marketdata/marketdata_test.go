package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/sfreiberg/webull/internal/signing"
	"github.com/sfreiberg/webull/internal/transport"
)

type fixture struct {
	t        *testing.T
	wantPath string
	body     []byte
	status   int
	gotQuery map[string][]string
	gotPath  string
	gotBody  []byte
}

func (f *fixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.gotPath, f.gotQuery = r.URL.Path, r.URL.Query()
	f.gotBody, _ = io.ReadAll(r.Body)
	if f.wantPath != "" && r.URL.Path != f.wantPath {
		f.t.Errorf("path = %q, want %q", r.URL.Path, f.wantPath)
	}
	if f.status != 0 {
		w.WriteHeader(f.status)
	}
	_, _ = w.Write(f.body)
}

type codedError struct{ code, msg string }

func (e *codedError) Error() string     { return e.msg }
func (e *codedError) ErrorCode() string { return e.code }

func load(t *testing.T, file string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func newClient(t *testing.T, wantPath, file string, status int) (*Client, *fixture) {
	t.Helper()
	f := &fixture{t: t, wantPath: wantPath, body: load(t, file), status: status}
	srv := httptest.NewTLSServer(f)
	t.Cleanup(srv.Close)
	c := New(&transport.Doer{
		HTTPClient: srv.Client(),
		Signer:     signing.New("k", "s"),
		DecodeError: func(r transport.Response) error {
			var p struct {
				Code string `json:"error_code"`
				Msg  string `json:"message"`
			}
			_ = json.Unmarshal(r.Body, &p)
			return &codedError{code: p.Code, msg: p.Msg}
		},
		Retry: transport.RetryPolicy{MaxAttempts: 1},
	}, strings.TrimPrefix(srv.URL, "https://"))
	return c, f
}

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }

func TestMillisDecodesNumberStringAndNull(t *testing.T) {
	var v struct {
		N Millis `json:"n"`
		S Millis `json:"s"`
		Z Millis `json:"z"`
		A Millis `json:"a"`
	}
	if err := json.Unmarshal([]byte(`{"n":1787860801043,"s":"1787860801043","z":null}`), &v); err != nil {
		t.Fatal(err)
	}
	want := time.UnixMilli(1787860801043).UTC()
	if !v.N.Equal(want) || !v.S.Equal(want) {
		t.Errorf("number=%v string=%v want %v", v.N, v.S, want)
	}
	if !v.Z.IsZero() || !v.A.IsZero() {
		t.Error("null and absent must both be zero")
	}
	if err := json.Unmarshal([]byte(`{"n":"yesterday"}`), &v); err == nil {
		t.Error("non-numeric must be rejected")
	}
	if b, _ := json.Marshal(v.N); string(b) != "1787860801043" {
		t.Errorf("marshal = %s", b)
	}
	if b, _ := json.Marshal(Millis{}); string(b) != "null" {
		t.Errorf("zero marshal = %s", b)
	}
}

func TestSnapshots(t *testing.T) {
	c, f := newClient(t, "/market-data/stocks/snapshots/list", "snapshots.json", 0)
	got, err := c.Snapshots(context.Background(), SnapshotsRequest{Symbols: []string{"AAPL", "SPY"}, ExtendedHours: true, Overnight: true})
	if err != nil {
		t.Fatal(err)
	}
	q := f.gotQuery
	if q["symbols"][0] != "AAPL,SPY" || q["category"][0] != "US_STOCK" || q["extend_hour_required"][0] != "true" || q["overnight_required"][0] != "true" {
		t.Errorf("query = %v", q)
	}
	if len(got) != 2 {
		t.Fatalf("got %d snapshots", len(got))
	}
	a := got[0]
	if a.Symbol != "AAPL" || !a.Price.Valid || !a.Price.Decimal.Equal(d("314.6288")) {
		t.Errorf("AAPL = %+v", a)
	}
	if a.LastTradeTime.IsZero() || a.QuoteTime.IsZero() || a.OvernightQuoteTime.IsZero() {
		t.Error("integer millisecond times did not decode")
	}
	if !a.OvernightPrice.Valid || !a.PERatio.Valid {
		t.Error("overnight and undocumented valuation fields should decode")
	}
}

func TestSnapshotsOmitsSessionFlagsWhenNotRequested(t *testing.T) {
	c, f := newClient(t, "", "snapshot_etf.json", 0)
	if _, err := c.Snapshots(context.Background(), SnapshotsRequest{Symbols: []string{"SPY"}, Category: USETF}); err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["category"][0] != "US_ETF" {
		t.Errorf("category = %v", f.gotQuery["category"])
	}
	for _, k := range []string{"extend_hour_required", "overnight_required"} {
		if _, ok := f.gotQuery[k]; ok {
			t.Errorf("%s sent when not requested", k)
		}
	}
}

func TestDepth(t *testing.T) {
	c, f := newClient(t, "/market-data/stocks/depths/list", "depth.json", 0)
	got, err := c.Depth(context.Background(), DepthRequest{Symbol: "AAPL", Levels: 1})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["depth"][0] != "1" || f.gotQuery["overnight_required"][0] != "false" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if len(got.Asks) != 1 || !got.Asks[0].Price.Equal(d("314.66")) || !got.Bids[0].Size.Equal(d("64")) || got.QuoteTime.IsZero() {
		t.Errorf("depth = %+v", got)
	}
}

func TestTicks(t *testing.T) {
	c, f := newClient(t, "/market-data/stocks/ticks/list", "ticks.json", 0)
	got, err := c.Ticks(context.Background(), TicksRequest{Symbol: "AAPL", Count: 30, Sessions: []TradingSession{Regular, AfterHours}})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["count"][0] != "30" || f.gotQuery["trading_sessions"][0] != "RTH,ATH" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if len(got.Ticks) == 0 {
		t.Fatal("no ticks")
	}
	first := got.Ticks[0]
	// Tick times are epoch milliseconds as strings on the wire.
	if first.Time.IsZero() || first.Time.Year() != 2026 || !first.Price.Equal(d("314.58")) || first.Side != TickNeutral || first.Session != Regular {
		t.Errorf("tick = %+v", first)
	}
}

func TestBarsRequestBodyAndDecoding(t *testing.T) {
	c, f := newClient(t, "/market-data/stocks/bars/list", "bars.json", 0)
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	got, err := c.Bars(context.Background(), BarsRequest{Symbols: []string{"AAPL", "MSFT"}, Timespan: Daily, Count: 3, Completed: true,
		Sessions: []TradingSession{Regular}, Start: start})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	_ = json.Unmarshal(f.gotBody, &body)
	if body["category"] != "US_STOCK" || body["timespan"] != "D" || body["count"] != float64(3) || body["real_time_required"] != false ||
		body["trading_sessions"] != "RTH" || body["start_time"] != float64(start.UnixMilli()) {
		t.Errorf("body = %v", body)
	}
	if _, has := body["end_time"]; has {
		t.Error("unset end_time must be omitted")
	}
	if len(got) != 2 || got[0].Symbol != "AAPL" || len(got[0].Bars) == 0 {
		t.Fatalf("bars = %+v", got)
	}
	b := got[0].Bars[0]
	if b.Time.IsZero() || !b.Open.Equal(d("310.545")) || !b.Volume.Equal(d("32363102")) {
		t.Errorf("bar = %+v", b)
	}
}

func TestBarsOmitsRealTimeFlagWhenNotCompleted(t *testing.T) {
	c, f := newClient(t, "", "bars.json", 0)
	if _, err := c.Bars(context.Background(), BarsRequest{Symbols: []string{"AAPL"}, Timespan: Daily}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(f.gotBody), "real_time_required") || strings.Contains(string(f.gotBody), "count") {
		t.Errorf("unset optionals sent: %s", f.gotBody)
	}
}

func TestFootprintsRequestAndNotSubscribed(t *testing.T) {
	c, f := newClient(t, "/market-data/stocks/footprints/list", "error_not_subscribed.json", http.StatusForbidden)
	_, err := c.Footprints(context.Background(), FootprintsRequest{Symbols: []string{"AAPL"}, Timespan: Minute5, Count: 3, Session: Regular})
	if !errors.Is(err, ErrNotSubscribed) {
		t.Fatalf("got %v, want ErrNotSubscribed", err)
	}
	var coded *codedError
	if !errors.As(err, &coded) || !strings.Contains(coded.msg, "FOOTPRINT") {
		t.Errorf("the product name must survive: %v", err)
	}
	if f.gotQuery["real_time_required"][0] != "true" || f.gotQuery["timespan"][0] != "M5" || f.gotQuery["trading_sessions"][0] != "RTH" {
		t.Errorf("query = %v", f.gotQuery)
	}
}

func TestImbalanceEndpoints(t *testing.T) {
	c, f := newClient(t, "", "error_not_subscribed.json", http.StatusForbidden)
	if _, err := c.ImbalanceBars(context.Background(), ImbalanceRequest{Symbol: "AAPL", Type: PreClose}); !errors.Is(err, ErrNotSubscribed) {
		t.Errorf("bars: %v", err)
	}
	if f.gotPath != "/market-data/stocks/noii-bars/list" || f.gotQuery["imbalance_action_type"][0] != "PRE_CLOSE" {
		t.Errorf("path=%s query=%v", f.gotPath, f.gotQuery)
	}
	if _, err := c.ImbalanceSnapshot(context.Background(), ImbalanceRequest{Symbol: "AAPL", Type: PreOpen}); !errors.Is(err, ErrNotSubscribed) {
		t.Errorf("snapshot: %v", err)
	}
	if f.gotPath != "/market-data/stocks/noii-snapshots/list" {
		t.Errorf("path = %s", f.gotPath)
	}
}

func TestOtherErrorsAreNotNotSubscribed(t *testing.T) {
	c, _ := newClient(t, "", "error_depth.json", http.StatusExpectationFailed)
	_, err := c.Depth(context.Background(), DepthRequest{Symbol: "AAPL", Levels: 10})
	if err == nil || errors.Is(err, ErrNotSubscribed) {
		t.Errorf("a parameter error must not classify as not subscribed: %v", err)
	}
}

func TestFundamentals(t *testing.T) {
	c, f := newClient(t, "/market-data/fundamentals/company-profiles/get", "company_profile.json", 0)
	cp, err := c.CompanyProfile(context.Background(), "AAPL", "")
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["category"][0] != "US_STOCK" || cp.CompanyName != "Apple Inc" || cp.Exchange != "NASDAQ" || len(cp.Industries) == 0 {
		t.Errorf("profile = %+v", cp)
	}

	c2, _ := newClient(t, "/market-data/fundamentals/analysis/ratings/get", "analyst_rating.json", 0)
	r, err := c2.AnalystRating(context.Background(), "AAPL", USStock)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Analysts.Equal(d("44")) || !r.StrongBuy.Equal(d("19")) || r.EffectiveFrom.IsZero() {
		t.Errorf("rating = %+v", r)
	}

	c3, _ := newClient(t, "/market-data/fundamentals/analysis/target-prices/get", "target_price.json", 0)
	tp, err := c3.TargetPrice(context.Background(), "AAPL", USStock)
	if err != nil {
		t.Fatal(err)
	}
	if !tp.Mean.Equal(d("324.45282")) || tp.Currency != "USD" || !tp.High.Equal(d("400")) {
		t.Errorf("target = %+v", tp)
	}
}

// assertCategoryErrors runs each call against a server answering with
// error_category.json and asserts the coded error propagates unchanged.
func assertCategoryErrors(t *testing.T, calls func(c *Client, ctx context.Context) map[string]func() error) {
	t.Helper()
	c, _ := newClient(t, "", "error_category.json", http.StatusExpectationFailed)
	for name, call := range calls(c, context.Background()) {
		var coded *codedError
		if err := call(); !errors.As(err, &coded) || coded.code != "UNSUPPORTED_CATEGORY" {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestErrorsPropagateFromEveryMethod(t *testing.T) {
	assertCategoryErrors(t, func(c *Client, ctx context.Context) map[string]func() error {
		return map[string]func() error{
			"Snapshots": func() error { _, e := c.Snapshots(ctx, SnapshotsRequest{Symbols: []string{"A"}}); return e },
			"Depth":     func() error { _, e := c.Depth(ctx, DepthRequest{Symbol: "A"}); return e },
			"Ticks":     func() error { _, e := c.Ticks(ctx, TicksRequest{Symbol: "A"}); return e },
			"Bars":      func() error { _, e := c.Bars(ctx, BarsRequest{Symbols: []string{"A"}, Timespan: Daily}); return e },
			"Footprints": func() error {
				_, e := c.Footprints(ctx, FootprintsRequest{Symbols: []string{"A"}, Timespan: Minute})
				return e
			},
			"ImbalanceBars":     func() error { _, e := c.ImbalanceBars(ctx, ImbalanceRequest{Symbol: "A", Type: PreOpen}); return e },
			"ImbalanceSnapshot": func() error { _, e := c.ImbalanceSnapshot(ctx, ImbalanceRequest{Symbol: "A", Type: PreOpen}); return e },
			"CompanyProfile":    func() error { _, e := c.CompanyProfile(ctx, "A", ""); return e },
			"AnalystRating":     func() error { _, e := c.AnalystRating(ctx, "A", ""); return e },
			"TargetPrice":       func() error { _, e := c.TargetPrice(ctx, "A", ""); return e },
		}
	})
}

func TestMillisZeroOnTheWireIsAbsent(t *testing.T) {
	var v struct {
		N Millis `json:"n"`
		S Millis `json:"s"`
	}
	if err := json.Unmarshal([]byte(`{"n":0,"s":"0"}`), &v); err != nil {
		t.Fatal(err)
	}
	if !v.N.IsZero() || !v.S.IsZero() {
		t.Error("a zero-filled timestamp must read as absent, not as 1970")
	}
}

func TestFootprintsDecode(t *testing.T) {
	c, _ := newClient(t, "", "footprints.json", 0)
	got, err := c.Footprints(context.Background(), FootprintsRequest{Symbols: []string{"AAPL"}, Timespan: Minute5})
	if err != nil {
		t.Fatal(err)
	}
	fp := got[0].Footprints[0]
	if fp.Time.IsZero() || !fp.Delta.Equal(d("200")) || !fp.BuyDetail["24.21"].Equal(d("500")) || fp.Session != Regular {
		t.Errorf("footprint = %+v", fp)
	}
}

func TestSnapshotsDefaultsCategory(t *testing.T) {
	c, f := newClient(t, "", "snapshots.json", 0)
	if _, err := c.Snapshots(context.Background(), SnapshotsRequest{Symbols: []string{"AAPL"}}); err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["category"][0] != "US_STOCK" {
		t.Errorf("default category = %v", f.gotQuery["category"])
	}
}
