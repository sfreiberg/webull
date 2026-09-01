package marketdata

import (
	"context"
	"testing"
)

func TestGainersLosersRequestAndDecoding(t *testing.T) {
	c, f := newClient(t, "/market-data/screeners/gainers-losers/list", "screener_stocks.json", 0)
	got, err := c.GainersLosers(context.Background(), GainersLosersRequest{Period: Rank1Day, SortBy: SortChangeRatio, Direction: Ascending})
	if err != nil {
		t.Fatal(err)
	}
	q := f.gotQuery
	if q["rank_type"][0] != "DAY_1" || q["category"][0] != "US_STOCK" || q["sort_by"][0] != "CHANGE_RATIO" || q["direction"][0] != "ASC" {
		t.Errorf("query = %v", q)
	}
	s := got[0]
	if s.Symbol != "SSM" || !s.ChangeRatio.Decimal.Equal(d("0.774254")) || !s.RelativeVolume.Decimal.Equal(d("5.724")) {
		t.Errorf("stock = %+v", s)
	}
	// Observed on the wire but undocumented.
	if s.Currency != "USD" || !s.PETTM.Decimal.Equal(d("-0.665351")) {
		t.Errorf("undocumented fields did not decode: %+v", s)
	}
	if s.Turnover.Valid {
		t.Error("an absent turnover must be null, not zero")
	}
}

func TestTopActive(t *testing.T) {
	c, f := newClient(t, "/market-data/screeners/top-actives/list", "screener_stocks.json", 0)
	got, err := c.TopActive(context.Background(), TopActiveRequest{Metric: ByRelativeVolume})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["rank_type"][0] != "RELATIVE_VOLUME_10D" {
		t.Errorf("query = %v", f.gotQuery)
	}
	if _, ok := f.gotQuery["sort_by"]; ok {
		t.Errorf("sort_by must be omitted when unset: %v", f.gotQuery)
	}
	if len(got) != 1 || !got[0].Volume.Decimal.Equal(d("34319359")) {
		t.Errorf("stocks = %+v", got)
	}
}

func TestHighDividend(t *testing.T) {
	c, f := newClient(t, "/market-data/screeners/high-dividend-ranks/list", "dividend_stocks.json", 0)
	got, err := c.HighDividend(context.Background(), HighDividendRequest{SortBy: SortYield, Direction: Descending})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["sort_by"][0] != "YIELD" || f.gotQuery["direction"][0] != "DESC" {
		t.Errorf("query = %v", f.gotQuery)
	}
	s := got[0]
	if s.Symbol != "TSLA" || !s.Yield.Decimal.Equal(d("0.1111")) || !s.Dividend.Decimal.Equal(d("0.25")) || s.ExDate.IsZero() {
		t.Errorf("stock = %+v", s)
	}
}

func TestWeek52HighLow(t *testing.T) {
	c, f := newClient(t, "/market-data/screeners/week52-high-low/list", "week52_stocks.json", 0)
	got, err := c.Week52HighLow(context.Background(), Week52Request{Rank: NewHigh})
	if err != nil {
		t.Fatal(err)
	}
	if f.gotQuery["rank_type"][0] != "NEW_HIGH" {
		t.Errorf("query = %v", f.gotQuery)
	}
	s := got[0]
	if !s.Week52Price.Decimal.Equal(d("12.5")) || !s.ChangeRatio52.Decimal.Equal(d("-0.0392")) {
		t.Errorf("stock = %+v", s)
	}
}

func TestMarketSectorsAndDetail(t *testing.T) {
	c, f := newClient(t, "/market-data/screeners/market-sectors/list", "sectors.json", 0)
	got, err := c.MarketSectors(context.Background(), MarketSectorsRequest{Aggregate: SectorVolume, Period: Sector5Day, PaginationKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	q := f.gotQuery
	if q["agg_type"][0] != "VOLUME" || q["period"][0] != "D5" || q["pagination_key"][0] != "k" {
		t.Errorf("query = %v", q)
	}
	sec := got.Sectors[0]
	if sec.ID != "7324" || !sec.Declined.Decimal.Equal(d("147")) || len(sec.Leaders) != 1 || sec.Leaders[0].Symbol != "HUBCZ" {
		t.Errorf("sector = %+v", sec)
	}
	if got.PaginationKey == "" {
		t.Error("pagination key did not decode")
	}

	c2, f2 := newClient(t, "/market-data/screeners/market-sectors/get", "sector_detail.json", 0)
	detail, err := c2.SectorDetail(context.Background(), SectorDetailRequest{SectorID: "6391", SortBy: SortChangeRatio})
	if err != nil {
		t.Fatal(err)
	}
	if f2.gotQuery["sector_id"][0] != "6391" {
		t.Errorf("query = %v", f2.gotQuery)
	}
	if detail.Name != "Energy - Fossil Fuels" || len(detail.Stocks) != 1 || !detail.Stocks[0].Close.Decimal.Equal(d("12.01")) {
		t.Errorf("detail = %+v", detail)
	}
	if detail.PaginationKey != "" {
		t.Error("an absent pagination key must decode empty")
	}
}

func TestScreenerListDecodesEitherShape(t *testing.T) {
	// The sandbox returns a bare array (covered above); Webull's formatter
	// documentation describes a {has_more, data} envelope, which must
	// decode too.
	c, _ := newClient(t, "", "screener_stocks_enveloped.json", 0)
	got, err := c.GainersLosers(context.Background(), GainersLosersRequest{Period: Rank1Day})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Symbol != "SSM" || !got[0].ChangeRatio.Decimal.Equal(d("0.774254")) {
		t.Errorf("enveloped stocks = %+v", got)
	}
}

func TestSectorsPageDecodesBareList(t *testing.T) {
	c, _ := newClient(t, "", "sectors_bare.json", 0)
	got, err := c.MarketSectors(context.Background(), MarketSectorsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Sectors) != 1 || got.Sectors[0].ID != "7324" || got.PaginationKey != "" {
		t.Errorf("bare sectors = %+v", got)
	}
}

func TestErrorsPropagateFromEveryScreenerMethod(t *testing.T) {
	assertCategoryErrors(t, func(c *Client, ctx context.Context) map[string]func() error {
		return map[string]func() error{
			"GainersLosers": func() error { _, e := c.GainersLosers(ctx, GainersLosersRequest{Period: Rank1Day}); return e },
			"TopActive":     func() error { _, e := c.TopActive(ctx, TopActiveRequest{}); return e },
			"HighDividend":  func() error { _, e := c.HighDividend(ctx, HighDividendRequest{}); return e },
			"Week52HighLow": func() error { _, e := c.Week52HighLow(ctx, Week52Request{}); return e },
			"MarketSectors": func() error { _, e := c.MarketSectors(ctx, MarketSectorsRequest{}); return e },
			"SectorDetail":  func() error { _, e := c.SectorDetail(ctx, SectorDetailRequest{SectorID: "1"}); return e },
		}
	})
}
