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
| **Trading** | Complete | Yes | Yes | 4b | Full lifecycle verified against sandbox |
| Order place / preview | Complete | Yes | Yes | 4b | Local validation before any request |
| Order replace / cancel | Complete | Yes | Yes | 4b | Keyed by `client_order_id` |
| Order query, history, open | Complete | Yes | – | 4b | History status lags a fresh cancel |
| Batch place | Unverified | Yes | – | 4b | Constraints enforced locally; not exercised live |
| Equity orders | Complete | Yes | Yes | 4b | Placed, replaced and cancelled in sandbox |
| Option orders | Partial | Yes | Yes | 4b | Single-leg previewed live; multi-leg strategies are Phase 5 |
| Futures orders | Planned | – | – | 5 | |
| Crypto orders | Planned | – | – | 5 | |
| Event-contract orders | Planned | – | – | 5 | |
| Advanced orders (OCO/OTO/OTOCO, trailing) | Planned | – | – | 5 | Combo types present in SDK enums |
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
| Order history freshness | Immediately after a cancel, `get` reports `CANCELLED` while `history` still reports `PENDING` |
| Preview response | Carries an undocumented `currency` field |
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
| 4 | Whether sandbox simulates market hours | Test harness design |
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
