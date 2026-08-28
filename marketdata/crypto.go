package marketdata

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/sfreiberg/webull/internal/query"
)

// CryptoSnapshot is the current state of a crypto pair. Sizes are in coins
// and may carry up to eight decimal places.
type CryptoSnapshot struct {
	Symbol       string `json:"symbol"`
	InstrumentID string `json:"instrument_id"`

	Price         decimal.NullDecimal `json:"price"`
	Open          decimal.NullDecimal `json:"open"`
	High          decimal.NullDecimal `json:"high"`
	Low           decimal.NullDecimal `json:"low"`
	Close         decimal.NullDecimal `json:"close"`
	PreClose      decimal.NullDecimal `json:"pre_close"`
	Change        decimal.NullDecimal `json:"change"`
	ChangeRatio   decimal.NullDecimal `json:"change_ratio"`
	LastTradeTime Millis              `json:"last_trade_time"`

	Bid       decimal.NullDecimal `json:"bid"`
	BidSize   decimal.NullDecimal `json:"bid_size"`
	Ask       decimal.NullDecimal `json:"ask"`
	AskSize   decimal.NullDecimal `json:"ask_size"`
	QuoteTime Millis              `json:"quote_time"`
}

// CryptoSnapshots returns the current state of crypto pairs such as "BTCUSD".
func (c *Client) CryptoSnapshots(ctx context.Context, symbols []string) ([]CryptoSnapshot, error) {
	q := query.New()
	q.SetList("symbols", symbols)
	q.Set("category", string(USCrypto))
	var out []CryptoSnapshot
	if err := c.get(ctx, "/market-data/crypto/snapshots/list", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CryptoBars returns candles for crypto pairs. Crypto bars carry no volume;
// Bar.Volume is zero.
func (c *Client) CryptoBars(ctx context.Context, req AssetBarsRequest) ([]Bars, error) {
	return c.bars(ctx, "/market-data/crypto/bars/list", req, USCrypto)
}
