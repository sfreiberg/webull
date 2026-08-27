# API compatibility matrix

Maps Webull OpenAPI capability to SDK status. This document is maintained for the life of the project and updated with every
phase that adds coverage.

Scope is the **US** market.

## Status vocabulary

| Status | Meaning |
|---|---|
| Complete | Implemented, tested, documented |
| Partial | Some operations implemented |
| Planned | Assigned to a phase, not yet started |
| Blocked | Cannot be implemented; reason required |
| Excluded | Deliberately out of scope; reason required |
| Unverified | Implemented against documentation but never exercised against a live endpoint |

`Unverified` exists because coverage is not the same as proof. Sandbox
credentials are available, so most work can now be exercised against a real
endpoint; anything that cannot — production-only behaviour, or an API whose
credentials are partner-gated — is marked `Unverified` rather than presented as
working.

## Summary

The transport layer and the Trading API — accounts, positions, instrument data
and orders — are implemented. Market data, streaming and Connect are not.

| Area | Status | Tests | Example | Phase | Notes |
|---|---|---|---|---|---|
| **Transport and signing** | Complete | Yes | – | 3 | HMAC-SHA256, verified against live sandbox |
| Request signing | Complete | Yes | – | 3 | Golden-vector tests plus live verification |
| Token lifecycle | Partial | Yes | – | 3 | `token_check_enabled` is false in sandbox, so untested in anger |
| Client token | Planned | – | – | 3 | Documented only; no SDK reference |
| Environments | Complete | Yes | – | 3 | Sandbox verified; production unverified (no keys) |
| **Accounts** | Complete | Yes | – | 4a | Verified against sandbox |
| Account list | Complete | Yes | – | 4a | |
| Balances | Complete | Yes | – | 4a | `open_margin_calls` is a list on the wire, not the documented string |
| Positions | Unverified | Yes | – | 4a | Decodes per the documented schema, but the sandbox account holds no positions so no live response has been seen. The SDK-only position-detail path has no documented equivalent and is not implemented |
| Activities | Complete | Yes | – | 4a | Keyset pagination via `last_activity_id`; `page_size` 1–200 verified |
| Trading instruments | Complete | Yes | – | 4a | Stocks, options, futures, crypto, event contracts; cursor pagination |
| **Trading** | Complete | Yes | Yes | 5b | Every asset class verified live; see sub-rows for sandbox limits |
| Order place / preview | Complete | Yes | Yes | 4b | Local validation before any request |
| Order replace / cancel | Complete | Yes | Yes | 4b | Keyed by `client_order_id`; a price-only replace verified live |
| Order query, history, open | Complete | Yes | – | 4b | History status lags a fresh cancel |
| Batch place | Unverified | Yes | – | 5b | Constraints enforced locally. **The sandbox account rejects it**: `Account not supported, please contact Webull` |
| Equity orders | Complete | Yes | Yes | 4b | Placed, replaced and cancelled in sandbox |
| Option orders | Complete | Yes | Yes | 5a | Single-leg placed live; vertical, iron condor, straddle and covered stock previewed live |
| Futures orders | Complete | Yes | – | 5b | Previewed live; the placing test skips during the exchange's daily break |
| Crypto orders | Complete | Yes | – | 5b | Placed and cancelled live; **the sandbox rejects every crypto preview** with a system error |
| Event-contract orders | Complete | Yes | – | 5b | Placed and cancelled live, DAY and GTC |
| Bracket orders (take-profit / stop-loss) | Complete | Yes | Yes | 5a | Placed, inspected and cancelled live; cancelling the master cancels the group |
| Trailing stops | Complete | Yes | – | 5a | Previewed live in the integration suite |
| OTO / OCO / OTOCO | Unverified | Yes | – | 5a | Implemented to the documented table. **The sandbox rejects all three** with `invalid combo_type` regardless of shape, contrary to the docs |
| **Market data (HTTP)** | Planned | – | – | 6 | |
| Snapshots, quotes, ticks, bars | Planned | – | – | 6 | Stocks, options, futures, crypto, events |
| Depth of book | Planned | – | – | 6 | Futures and event contracts |
| Footprint | Planned | – | – | 6 | Stocks and futures |
| NOII | Planned | – | – | 6 | |
| Instruments and contracts | Planned | – | – | 6 | Market-data reference endpoints, distinct from the trading instrument lookups |
| Fundamentals and financials | Planned | – | – | 6 | |
| Funds | Planned | – | – | 6 | |
| Screeners | Planned | – | – | 6 | |
| Watchlists | Planned | – | – | 6 | |
| Corporate actions, calendars | Planned | – | – | 6 | |
| News | Planned | – | – | 6 | Documented; absent from all SDKs |
| **Connect API / OAuth** | Planned | – | – | 7 | Documented only; credentials are partner-gated |
| **gRPC trade events** | Planned | – | – | 8 | `.proto` available; unblocked |
| **MQTT market data** | Planned | – | – | 9 | `.proto` available; no sandbox host published |
| **Broker API** | Excluded | – | – | – | Out of scope; see below |
| **FIX** | Excluded | – | – | – | See below |

## Exceptions and scope decisions

### FIX — Excluded

Webull documents a FIX interface. It is out of scope for this SDK.

A FIX engine is a distinct protocol stack with its own session management,
sequence-number recovery and persistence requirements. It does not share
meaningful implementation with an HTTP/gRPC/MQTT client, and bundling one into
this SDK would roughly double its surface for an audience that overwhelmingly
already uses a dedicated FIX library.

This is a deliberate exclusion, not an oversight, and reversing it later is a
clean addition rather than a redesign.

### Broker API — Excluded

115 documented reference pages covering account opening, ACH and wire funding,
cash journals, agreements, document handling, and a dedicated event stream.

Excluded by project decision. The reasoning:

- No official SDK implements any of it, so there is no reference implementation
  to check behaviour against.
- Access requires an enterprise relationship with Webull, so none of it could be
  tested by this project.
- It is larger than the entire individual Trading and Market Data surface
  combined.

This is a deliberate scope decision rather than a technical block. A reasonable
case exists for including it anyway — most SDK users lacking credentials is not
by itself a reason to omit an API. The judgement here is that shipping endpoints that
move money and have never been executed once would be worse than not shipping
them. Adding it later is additive and needs no redesign, since its
authentication and domain model are separate anyway.

### Previous-generation gRPC quotes — Likely excluded

`quotes.proto` and `gateway.proto` describe a gRPC market-data transport in the
older SDKs, superseded by MQTT in the current generation. To be confirmed during
Phase 9 rather than assumed.

## Recorded decisions

| Decision | Choice | Rationale |
|---|---|---|
| Decimal representation | `github.com/shopspring/decimal`, `NullDecimal` + `omitzero` for optionals in both directions | [wire-format.md](discovery/wire-format.md#recommendation-githubcomshopspringdecimal) |
| Broker API | Excluded | Untestable, no reference implementation, larger than the rest of the SDK |
| FIX | Excluded | Unrelated protocol stack; served by dedicated engines |
| Region scope | US only | Other regions are a configuration addition, not a redesign |
| Package layout | Root package at repo root | [package-layout.md](discovery/package-layout.md) |
| Sandbox support | Required | Hosts known for trading and events; market data unconfirmed |
| Endpoint path scheme | Documented (`/trading/*`) | Both schemes are live aliases; the documented one is the source of truth — [open-questions.md](discovery/open-questions.md#1-the-documentation-and-the-sdks-describe-different-api-generations) |
| Connect API | In scope, likely unverifiable | No new endpoints; cost is a host and a credential — [connect-api.md](discovery/connect-api.md#implementation-cost) |

## Observed API behaviour

Established against the live sandbox and relied on by the implementation.

| Behaviour | Detail |
|---|---|
| Documented types that are wrong on the wire | `unit` (futures) and `category_id` (events) are integers, not strings; `open_margin_calls` is an array, not a string; event categories are `ELECTIONS` and `COMMODITIES`, not the documented `POLITICS` and `TRANSPORTATION` |
| Undocumented fields | `single_stock_etf`, `inverse_etf` on stock profiles; `init_expiration_date` on option contracts |
| Instrument page size | Fixed by the server at 1000; `pagination_key` is absent on the last page |
| `support_trading_session` | Documented optional; **required** for US equity orders (417 without it), optional for options. The SDK defaults it to `CORE` |
| `client_order_id` length | Documented as at most 32; the server requires **10 to 40** |
| Explicit `null` on an optional order field | Accepted |
| `client_order_id` lifetime | Consumed once an order is accepted (`OPENAPI_TRADE_PLACE_ORDER_REPEAT` on reuse); a rejected placement does not consume it |
| Option leg symbol | The OCC root (`SPXW`), not the underlying (`SPX`), which is rejected |
| Group receipt | A combo placement returns `combo_order_id` and `client_combo_order_id` only; sub-orders are looked up by their own `client_order_id`, each reporting its role as `combo_type` |
| Group cancellation | Cancelling an unfilled master cancels every order in the group; the children then report `OPENAPI_ORDER_NOT_FOUND`. Once the master has filled, the exits are working orders and must be cancelled themselves; `CancelCombo` attempts every order and treats `OPENAPI_ORDER_NOT_FOUND` and `OPENAPI_ORDER_CAN_NOT_BE_CANCEL` as already done |
| Multi-leg validation | Preview accepts a `VERTICAL` with one leg, so leg counts are not checked server-side at preview |
| OTO / OCO / OTOCO in sandbox | Rejected with `OPENAPI_PARAM_ERR: invalid combo_type, value: ["MASTER","OTO"]` for every documented shape, while `MASTER` and `OTO` are accepted as lone roles |
| Order history freshness | Immediately after a cancel, `get` reports `CANCELLED` while `history` still reports `PENDING` |
| Preview response | Carries an undocumented `currency` field |
| Rate limiting | `GET /trading/accounts/list` returned 429 `TOO_MANY_REQUESTS` after roughly eight calls in quick succession across consecutive test runs; the integration suite now fetches it once per run |
| Asset-class rules | Not in the OpenAPI definition; taken from the trading FAQ and guides, checked against the sandbox, and enforced locally: options no trailing stops; futures and crypto BUY/SELL only; crypto MARKET/LIMIT/STOP_LOSS_LIMIT with ≤8 decimal places; event contracts LIMIT-only, quantity ≤2 decimal places, `event_outcome` required |
| Option order types | The FAQ says options do not support MARKET orders and that sells are DAY-only. The sandbox previews a market order on a single-leg call, a vertical and an iron condor, and a GTC option sell, so neither is enforced by the SDK. A trailing stop on an option is rejected with `invalid trailing_type` |
| Crypto preview | Every shape returns 417 `OPENAPI_SYSTEM_ERROR` in the sandbox; placement of the same order succeeds. Crypto `order_id`s have a different format (`CO0382…`) and crypto order records carry no `fees` or `commission` |
| Crypto minimum | `OPENAPI_CRYPTO_ORDER_BUY_SELL_LIMIT_MINIMUM`: $2.00 per order |
| Batch placement | Rejected for the sandbox margin account with `Account not supported, please contact Webull` — a feature flag rather than a validation error |
| Futures hours | Placement outside the contract's session returns `OPENAPI_FUTURES_CAN_NOT_TRADING_FOR_NON_TRADING_HOURS`; previews succeed at any time |
| Event contracts | The guide says DAY only, but GTC places successfully; `MARKET` is rejected; preview checks neither account class nor position, so a sell-to-close with no position previews fine |
| Accounts per asset class | Futures and crypto orders belong on the `FUTURES` and `CRYPTO` accounts, events on `EVENTS_CASH`. Preview accepts them on the margin account too, with different fees |
| Market hours | Enforced for placement: a `DAY` order after the regular session ends returns 417 `OPENAPI_DAY_ORDER_NOT_ALLOWED_AFT_CORE_TIME_LIMIT`. Previews, lookups and `GTC` placements work around the clock |
| Parameter validation status | **HTTP 417**, not 400, with code `OPENAPI_PARAM_ERR` |
| Two error shapes | Application errors return `error_code` + `message`; gateway errors, such as an unrouted path, return only `error_msg` |
| Wrong environment or unprovisioned key | `404 Route Not Found`, not 401 |
| Missing credentials | 401 with `MISSING_APP_KEY` |
| Account not accessible | 403 with `ACCOUNT_ACCESS_DENIED` |
| Pagination bounds | Per endpoint: open orders require `page_size` 10–100; cash activities accept 1–200 and reject 201 with a 417 |
| Decimal encoding | Confirmed on the wire: `"total_cash_balance":"1000000.00"` — a string, with trailing zeros preserved |

## Sandbox validation backlog

Sandbox credentials are available; production credentials are not. Items below
are acceptance criteria that remain unverified, tracked here so the gap stays
visible rather than being quietly forgotten. Struck-through items have been
resolved and are kept for a release or two so the answers are discoverable.

| # | Item | Blocks |
|---|---|---|
| ~~1~~ | ~~Which endpoint path scheme the server honours~~ — **resolved: both, they are aliases. Using the documented scheme.** | – |
| 2 | Whether `data-api.sandbox.webull.com` exists, and whether MQTT has any sandbox | Phase 6 / 9 |
| ~~3~~ | ~~Whether sandbox credentials are separate from production~~ — **resolved: yes. A sandbox key 404s every path in production.** | – |
| ~~4~~ | ~~Whether sandbox simulates market hours~~ — **resolved: yes, for order placement.** A `DAY` order in the `CORE` session is rejected outside regular hours with `OPENAPI_DAY_ORDER_NOT_ALLOWED_AFT_CORE_TIME_LIMIT`; `GTC` orders and previews are accepted at any time. Integration tests place GTC orders | – |
| ~~5~~ | ~~Confirm the server accepts our HMAC-SHA256 signature~~ — **resolved: accepted, with and without query parameters.** | – |
| 6 | Whether `token_check_enabled` is true in production (it is **false** in sandbox) | when production keys exist |
| 7 | Whether MQTT port 1883 or 8883 is preferred, and TLS expectations | Phase 9 |
| 8 | Whether streaming requires its own token | Phase 9 |
| 9 | Timestamp formats per endpoint | Phase 4 |
| 10 | Display vs Non-Display entitlement behaviour on 403 | Phase 6 |
| ~~11~~ | ~~Whether optional order fields accept an explicit `null`~~ — **resolved: accepted.** | – |
| 12 | Whether position `last_price`, `cost_price` and `unrealized_profit_loss` are ever absent; modelled as always present per the docs | when a position exists |
| 13 | The wire form of a finite `day_trades_left`; only `"UNLIMITED"` has been observed | when a cash account is available |
| 14 | Batch place against the sandbox, and whether a partially failed batch returns 200 | Phase 5 |
| 15 | Whether `OrderHistory` reconciles to the cancelled status after a delay | Phase 5 |
| 16 | The wire form of `filled_time_at` on a filled order; decoded as RFC 3339 like `place_time_at` but never observed | when an order fills |
| 17 | Whether OTO, OCO and OTOCO groups work in production, or need a shape the sandbox does not reveal | when production keys exist, or via Webull support |
| 18 | Which multi-leg strategies the server validates leg counts for at placement | later |
| 19 | Futures placement lifecycle, once run inside a trading session (previews verified; placement blocked by the daily break when tested) | next run during CME hours |
| 20 | Batch placement: the sandbox account returns `Account not supported, please contact Webull` (417 `OPENAPI_PARAM_ERR`), so it cannot be verified here | production keys, or Webull support |
| 21 | Whether crypto previews ever work in the sandbox | – |
| 22 | Whether option market orders and GTC option sells, which preview, are also accepted at placement; the FAQ says neither is | a run during options hours, with a position to sell |
