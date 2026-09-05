# Completeness audit — 2026-09-04 (Milestone 11)

The implementation compared against three sources, independently inventoried:

1. **Official documentation**: every page in `developer.webull.com/apis/llms.txt`
   (166 HTTP endpoints across four API products, plus gRPC and MQTT), fetched
   through the `.md` mirror on 2026-09-04.
2. **Python SDK**: `webull-inc/webull-openapi-python-sdk` @ `97ef8eb`
   (2026-08-31) — the successor to `openapi-python-sdk`, archived 2026-06-22.
3. **Java SDK**: `webull-inc/webull-openapi-java-sdk` @ `6ba9970` (v1.1.21,
   2026-08-31) — the successor to `openapi-java-sdk`, archived 2026-06.

## Result

The four documented API products break down as:

| Product | Documented | SDK position |
|---|---|---|
| Trading API + Market Data + Streaming + Trade Events | 87 HTTP + 2 gRPC subscription types + MQTT | **All implemented** (one gap found and filled: access tokens; two documented exceptions: news, single-symbol bars — below) |
| Connect API | 2 | All implemented (Phase 7) |
| Market Data Display Solution | 27 | Excluded — separate product (below) |
| Broker API | 50 HTTP + 11 gRPC event modules | Excluded — see COMPATIBILITY.md |

Field-level drift was also checked against the docs changelog: `position_intent`
(2026-03-28), instrument margin ratios (2026-06-06), the `order_id` event field
(2026-05-16), the `place_time`→`place_time_at` deprecation (2026-08-20),
`trading_sessions`, `overnight_required`, MQTT `grab`/`depth`, the `notice`
topic, and the gRPC subscribe bitmask are all current in the SDK.

## Gap found and filled

**Access tokens** (`/auth/tokens/create`, `/auth/tokens/check`): the token
flow for deployments where `token_check_enabled` is true. Implemented as
`Client.CreateAccessToken` / `Client.CheckAccessToken` with
`Config.AccessToken` carrying the `x-access-token` header, and **verified
live**: the sandbox issued a `NORMAL` token with the documented 15-day expiry
(the test environment skips SMS verification).

## Documented exceptions (core product)

- **News** (`news/summaries/get`): the one documented endpoint is a
  Server-Sent Events stream whose reference page contradicts the rest of the
  platform — it demands the raw app secret as a header, names HMAC-SHA1 where
  every verified endpoint uses HMAC-SHA256, requires an `x-access-token` with
  no stated linkage to the token flow, and documents no stream termination.
  It 404s in the sandbox and no official SDK implements it, so none of those
  contradictions can be resolved without inventing behaviour. Blocked.
- **Single-symbol bars** (`GET /market-data/stocks/bars/get`): the reference
  page (`reference/bars`) 404s; the path is only inferable from its Display
  Solution twin. The documented multi-symbol `POST bars/list` covers the
  functionality. Not implemented until the page exists.

## Market Data Display Solution — excluded

A parallel market-data product added 2026-05-30 (stocks) and 2026-08-14
(event contracts): 27 endpoints on `us-global-openapi.uat.webullbroker.com`,
authenticated with **client tokens** (`/auth/client-tokens/create|refresh`,
an `access_token` header) rather than request signing — aimed at
fully-disclosed brokers displaying data to their end users.

This resolves a set of long-standing Blocked rows: corporate actions,
security logos, and the event-contract display endpoints (markets, live
data, game stats, milestones, sports filters) are documented **only** under
this product's paths, which is why they 404 on the core hosts. They were
never missing core endpoints; they belong to a product this SDK's app-key
credentials cannot reach (the documented host is UAT-only). Excluded as a
product, like the Broker API; adding it later is additive.

## Official SDK surface deliberately not mirrored

Both successor SDKs ship paths absent from the reference documentation. Per
the recorded endpoint-scheme decision, the documented paths are the source
of truth, so these stay out:

- `/trading/orders/executions/list`, position details, `/trade/calendar`
  (a stub in the Java SDK — it returns an empty list without a request),
  `/market-data/eod-bars`, `/instrument/corp-action`, batch-bars, stock
  `quotes` (the documented `depths/list` covers it), and the legacy
  unprefixed generation.
- The previous-generation gRPC quotes gateway: removed from the Python
  successor entirely; shipped as unused stubs in Java. Confirms the Phase 9
  decision to exclude it.
