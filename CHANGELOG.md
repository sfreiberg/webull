# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Until 1.0.0 the public API may change in any release. Breaking changes are
called out explicitly.

## [Unreleased]

### Added

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
- Market data, streaming and the Connect API are not implemented yet. The
  Broker API and FIX are out of scope; see
  [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md).

[Unreleased]: https://github.com/sfreiberg/webull/commits/main
