// Package marketdata provides access to Webull's Market Data API over HTTP:
// snapshots, quotes, ticks, bars and reference data.
//
// Obtain a Client from the root package:
//
//	client, err := webull.NewClient(webull.Config{...})
//	snaps, err := client.MarketData.Snapshots(ctx, marketdata.SnapshotsRequest{Symbols: []string{"AAPL"}})
//
// All methods take a context.Context and are safe for concurrent use.
//
// # Asset classes
//
// Stocks are the primary surface. Options (OptionSnapshots, with greeks),
// futures (FuturesSnapshots and friends), crypto (CryptoSnapshots) and event
// contracts (EventSnapshots, with a book per outcome) have their own methods,
// since their fields differ. Bars share one type across all of them.
//
// The sandbox serves futures data for a single symbol, "MESmain"; the
// display-solution event endpoints (per-market snapshots, live data, game
// stats) do not exist in the sandbox at all and are not implemented.
//
// # Fundamentals
//
// Company reference data covers profiles, analyst consensus, financial
// statements (BalanceSheets, IncomeStatements, CashFlows), indicators,
// earnings and dividend calendars, capital flow, regulatory filings, EPS
// forecasts and industry comparison. The sandbox answers the
// financial-statement and industry-comparison endpoints with empty results
// for every symbol; the rest serve real data.
//
// # Entitlements
//
// Market data is sold by product. A request for data the key is not
// subscribed to fails with ErrNotSubscribed, wrapping a *webull.APIError whose
// message names the product needed, such as "please subscribe to FOOTPRINT".
// Depth of book is limited to the levels the key is entitled to; asking for
// more is rejected as an invalid parameter. Futures depth and all footprint
// data need their own subscriptions.
//
// # Hosts
//
// Webull's SDKs route market data to a data-api host. That host does not
// answer HTTP requests - it is the MQTT streaming broker - and HTTP market
// data is served by the same host as trading. This package uses the latter.
//
// # Time
//
// The API transmits time in three forms: epoch milliseconds as a JSON number,
// epoch milliseconds as a string, and ISO 8601 with a "+0000" offset that the
// standard RFC 3339 layout does not accept. Millis and Time decode each of
// them into a time.Time.
//
// # Numbers
//
// Prices, sizes and ratios are decimal strings and are decoded as
// decimal.Decimal, or decimal.NullDecimal where the field may be absent -
// most snapshot fields are, outside a trading session. No field is a float.
package marketdata
