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

`Unverified` exists because this project currently has no API credentials. Any
coverage claim not exercised against a real endpoint is marked as such rather
than being presented as working.

## Summary

Nothing is implemented yet. Phase 1 produced the inventory below; implementation
begins in Phase 3.

| Area | Status | Tests | Example | Phase | Notes |
|---|---|---|---|---|---|
| **Transport and signing** | Planned | – | – | 3 | HMAC-SHA256, centralised |
| Request signing | Planned | – | – | 3 | Algorithm documented |
| Token lifecycle | Planned | – | – | 3 | Conditional on `/openapi/config` |
| Client token | Planned | – | – | 3 | Documented only; no SDK reference |
| Environments | Partial | – | – | 3 | Production hosts known; sandbox host unknown |
| **Accounts** | Planned | – | – | 4 | |
| Account list | Planned | – | – | 4 | |
| Balances | Planned | – | – | 4 | |
| Positions | Planned | – | – | 4 | Including position detail |
| Activities | Planned | – | – | 4 | |
| **Trading** | Planned | – | – | 4 | |
| Order place / preview | Planned | – | – | 4 | |
| Order replace / cancel | Planned | – | – | 4 | |
| Order query, history, open | Planned | – | – | 4 | |
| Batch place | Planned | – | – | 4 | |
| Equity orders | Planned | – | – | 4 | |
| Option orders | Planned | – | – | 5 | Dedicated option order endpoints |
| Futures orders | Planned | – | – | 5 | |
| Crypto orders | Planned | – | – | 5 | |
| Event-contract orders | Planned | – | – | 5 | |
| Advanced orders (OCO/OTO/OTOCO, trailing) | Planned | – | – | 5 | Combo types present in SDK enums |
| **Market data (HTTP)** | Planned | – | – | 6 | |
| Snapshots, quotes, ticks, bars | Planned | – | – | 6 | Stocks, options, futures, crypto, events |
| Depth of book | Planned | – | – | 6 | Futures and event contracts |
| Footprint | Planned | – | – | 6 | Stocks and futures |
| NOII | Planned | – | – | 6 | |
| Instruments and contracts | Planned | – | – | 6 | |
| Fundamentals and financials | Planned | – | – | 6 | |
| Funds | Planned | – | – | 6 | |
| Screeners | Planned | – | – | 6 | |
| Watchlists | Planned | – | – | 6 | |
| Corporate actions, calendars | Planned | – | – | 6 | |
| News | Planned | – | – | 6 | Documented; absent from all SDKs |
| **Connect API / OAuth** | Planned | – | – | 7 | Documented only; no SDK reference |
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
| Decimal representation | `github.com/shopspring/decimal` | [wire-format.md](discovery/wire-format.md#recommendation-githubcomshopspringdecimal) |
| Broker API | Excluded | Untestable, no reference implementation, larger than the rest of the SDK |
| FIX | Excluded | Unrelated protocol stack; served by dedicated engines |
| Region scope | US only | Other regions are a configuration addition, not a redesign |
| Package layout | Root package at repo root | [package-layout.md](discovery/package-layout.md) |
| Sandbox support | Required | Hosts known for trading and events; market data unconfirmed |

## Sandbox validation backlog

This project has no API credentials. Every item below is an acceptance criterion
deferred until credentials exist, tracked here so the gap stays visible rather
than being quietly forgotten.

| # | Item | Blocks |
|---|---|---|
| 1 | Which endpoint path scheme the server honours — documented or SDK | Phase 3 |
| 2 | Whether `data-api.sandbox.webull.com` exists, and whether MQTT has any sandbox | Phase 3 |
| 3 | Whether sandbox credentials are separate from production | Phase 3 |
| 4 | Whether sandbox simulates market hours | Test harness design |
| 5 | Whether the server still accepts HMAC-SHA1 signatures | Phase 3 |
| 6 | Whether `token_check_enabled` is true for US production | Phase 3 |
| 7 | Whether MQTT port 1883 or 8883 is preferred, and TLS expectations | Phase 9 |
| 8 | Whether streaming requires its own token | Phase 9 |
| 9 | Timestamp formats per endpoint | Phase 4 |
| 10 | Display vs Non-Display entitlement behaviour on 403 | Phase 6 |
| 11 | Whether optional order fields accept an explicit `null` or must be omitted | Phase 4 |
