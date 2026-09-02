# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Until 1.0.0 the public API may change in any release. Breaking changes are
called out explicitly.

## [Unreleased]

### Fixed

- Decimal fields now decode Webull's absent forms — `null` and an empty
  string — to an absent or zero value. Webull returns `""` for an unreported
  number (such as an option greek outside market hours), which previously
  failed to decode. The SDK's decimal types embed shopspring's, so their
  methods are unchanged.

### Added

- `streaming` package: real-time market data over MQTT, reachable as
  `client.Streaming`. `Connect` opens the broker connection; `Subscribe`
  and `Unsubscribe` register interest over the signed HTTP endpoints; `Recv`
  returns typed snapshots, quotes, ticks and their event-contract variants.
  A dropped connection reconnects and replays its subscriptions.
  Authentication is enforced at `Subscribe` (`ErrSubscribeFailed`), since
  the broker accepts the MQTT connection for any well-formed key. The
  vendored `message.proto` and its generated bindings are committed with
  their Apache-2.0 provenance.
- `events` package: real-time trade events over gRPC, reachable as
  `client.Events`. `Subscribe` blocks until the server acknowledges, so a
  returned `Stream` is live; `Recv` handles heartbeats, reconnects on
  transient failures with a fixed delay, and renews an expired
  subscription. Order events are fully typed; authentication rejection and
  the server's connection limit are `ErrAuthFailed` and
  `ErrConnectionLimit`. The vendored `events.proto` and its generated
  bindings are committed with their Apache-2.0 provenance.
- Fund reference data: `FundBrief`, `FundAllocations`, `FundDividends`,
  `FundFiles`, `FundHoldings`, `FundNetValues`, `FundPerformance`,
  `FundRatings` and `FundSplits`.
- Screeners: `GainersLosers`, `TopActive`, `HighDividend`, `Week52HighLow`,
  `MarketSectors` and `SectorDetail`, with typed rank, sort and period
  values.
- Watchlists: the full lifecycle — `Watchlists`, `CreateWatchlist`,
  `UpdateWatchlist`, `DeleteWatchlist`, `WatchlistInstruments` and
  add/remove/update of instruments. A mutation Webull reports unsuccessful
  is `marketdata.ErrWatchlistFailed`.
- Fundamentals reference data: financial statements (`BalanceSheets`,
  `IncomeStatements`, `CashFlows`), `FinancialIndicators`, `FinancialAlert`,
  `CapitalFlows`, `DividendCalendar`, `EarningsCalendar`, `Filings`,
  `ForecastEPS` and `IndustryComparison`.
- Market data for options (with greeks), futures, crypto and event contracts:
  snapshots, ticks, bars, and depth where the asset class has it.
- `marketdata` package: stock snapshots, depth, ticks, bars, footprints and
  auction imbalances, plus company profiles and analyst consensus, reachable
  as `client.MarketData`. Entitlement failures are `marketdata.ErrNotSubscribed`.
- Futures, crypto and event-contract orders, with each asset class's
  permitted sides, order types, times in force and quantity precision
  enforced before any request is sent.
- Order groups: `Bracket`, `OTO`, `OCO` and `OTOCO` build a `Combo`, placed
  with `PlaceCombo`, previewed with `PreviewCombo` and cancelled with
  `CancelCombo`. Webull's per-role rules are enforced locally.
- Multi-leg option strategies: every documented `OptionStrategy`, with stock
  legs for covered and collar strategies.
- Orders: `PreviewOrder`, `PlaceOrder`, `ReplaceOrder`, `CancelOrder`,
  `Order`, `OpenOrders`, `OrderHistory` and `PlaceOrders`. Orders are
  validated locally before any request; a `ClientOrderID` is generated and
  written back before sending so a lost response can be reconciled;
  `PlaceOrder` never retries. `LegFromSymbol` builds an option leg from an
  OCC symbol.
- `trade` package: accounts, balances, positions, cash activities, and
  reference data for stocks, options, futures, crypto and event contracts,
  reachable as `client.Trade`. Monetary values are `decimal.Decimal`, with
  `decimal.NullDecimal` where Webull may omit a field.
- `Client`, `Config` and `Environment`: the SDK entry point, with sandbox and
  production host resolution and per-service endpoint overrides.
- Webull request signing (HMAC-SHA256), verified against the live sandbox.
- `APIError` with sentinel errors for authentication, permission, invalid
  request, not found, rate limiting and server failures, usable with
  `errors.Is` and `errors.As`.
- Conservative retry policy: idempotent requests only, never `POST`, and
  never on a rate limit. `APIError.RetryAfter` carries Webull's requested
  delay so the caller can decide how long to wait.
- Redirects are refused (`ErrRedirectNotAllowed`) so signature headers are
  never forwarded to another host. Oversized responses are reported as
  `ErrResponseTooLarge` rather than silently truncated.
- `Client.TokenCheckEnabled` reports whether a deployment requires an access
  token in addition to the signature.
- API compatibility matrix and discovery documentation covering the Webull US
  OpenAPI surface, authentication, streaming protocols and wire format.
- Continuous integration: tests on Go 1.27 and 1.26, race detector, linting,
  vulnerability scanning, secret scanning, and an enforced 90% coverage floor.
- `Version` and `UserAgent` for identifying the SDK in outgoing requests.

### Notes

- Endpoint paths follow Webull's documented scheme (`/trading/...`). The
  `/openapi/...` paths used by Webull's own SDKs are live aliases of the same
  handlers and are not implemented.
- News is not implemented: the one documented endpoint is a Server-Sent
  Events stream that does not exist in the sandbox. The Connect API is not
  implemented yet. The Broker API and FIX are out of scope; see
  [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md).

[Unreleased]: https://github.com/sfreiberg/webull/commits/main
