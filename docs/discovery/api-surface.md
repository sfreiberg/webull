# API surface

Every endpoint recovered from the current official SDK, grouped by area. These
are distinct path literals; a few carry more than one operation, distinguished
by request body, so this is a map of the surface rather than a list of the Go
methods we will write.

**Read this alongside [open-questions.md](open-questions.md#1-the-documentation-and-the-sdks-describe-different-api-generations).**
The paths below are what the current SDK calls. The official reference
documentation describes a *different* path scheme for the same operations. Which
scheme the server honours is unverified and gates Phase 3. This inventory remains
valid as a map of *capabilities* either way; only the spelling is in question.

## Coverage gap between docs and SDK

The reference documentation describes noticeably more surface than the SDKs
implement. Two areas — the Connect API and the Broker API — are documented in
full but appear in no official SDK in any language, so implementing them means
working from prose with nothing to check behaviour against. That risk is why the
Broker API was excluded; see [../COMPATIBILITY.md](../COMPATIBILITY.md).

## Endpoints by area

### Accounts

- `/openapi/account/list`

### Balances and positions

- `/openapi/assets/balance`
- `/openapi/assets/positions`

**Position detail**

- `/openapi/assets/position/details`

### Authentication

**Tokens**

- `/openapi/auth/token/check`
- `/openapi/auth/token/create`
- `/openapi/auth/token/refresh`

### Client configuration

- `/openapi/config`

### Fundamentals

**Financials**

- `/openapi/fundamentals/financial/alert`
- `/openapi/fundamentals/financial/balance-sheet`
- `/openapi/fundamentals/financial/cash-flow`
- `/openapi/fundamentals/financial/income`
- `/openapi/fundamentals/financial/indicators`

**Funds**

- `/openapi/fundamentals/fund/allocation`
- `/openapi/fundamentals/fund/brief`
- `/openapi/fundamentals/fund/dividends`
- `/openapi/fundamentals/fund/files`
- `/openapi/fundamentals/fund/holdings`
- `/openapi/fundamentals/fund/net-value`
- `/openapi/fundamentals/fund/performance`
- `/openapi/fundamentals/fund/rating`
- `/openapi/fundamentals/fund/splits`

**Stocks**

- `/openapi/fundamentals/stock/capital-flow`
- `/openapi/fundamentals/stock/dividend-calendar`
- `/openapi/fundamentals/stock/earnings-calendar`
- `/openapi/fundamentals/stock/filings`
- `/openapi/fundamentals/stock/forecast-eps`
- `/openapi/fundamentals/stock/industry-comparison`

### Instruments and reference data

**Analyst data**

- `/openapi/instrument/analyst/rating`
- `/openapi/instrument/analyst/target-price`

**Company data**

- `/openapi/instrument/company/profile`

**Crypto**

- `/openapi/instrument/crypto/list`

**Event contracts**

- `/openapi/instrument/event/categories`
- `/openapi/instrument/event/events`
- `/openapi/instrument/event/market/list`
- `/openapi/instrument/event/series/list`

**Futures**

- `/openapi/instrument/futures/by-code`
- `/openapi/instrument/futures/list`
- `/openapi/instrument/futures/product-classes`
- `/openapi/instrument/futures/products`

**Options**

- `/openapi/instrument/option/contracts`

**Stocks**

- `/openapi/instrument/stock/list`

### Market data

**Crypto**

- `/openapi/market-data/crypto/bars`
- `/openapi/market-data/crypto/snapshot`

**Event contracts**

- `/openapi/market-data/event/bars`
- `/openapi/market-data/event/depth`
- `/openapi/market-data/event/snapshot`
- `/openapi/market-data/event/tick`

**Futures**

- `/openapi/market-data/futures/bars`
- `/openapi/market-data/futures/depth`
- `/openapi/market-data/futures/footprint`
- `/openapi/market-data/futures/snapshot`
- `/openapi/market-data/futures/tick`

**Options**

- `/openapi/market-data/option/bars`
- `/openapi/market-data/option/snapshot`
- `/openapi/market-data/option/tick`

**Screeners**

- `/openapi/market-data/screener/52whl`
- `/openapi/market-data/screener/gainers-losers`
- `/openapi/market-data/screener/high-dividend`
- `/openapi/market-data/screener/market-sectors`
- `/openapi/market-data/screener/market-sectors-detail`
- `/openapi/market-data/screener/top-active`

**Stocks**

- `/openapi/market-data/stock/bars`
- `/openapi/market-data/stock/batch-bars`
- `/openapi/market-data/stock/footprint`
- `/openapi/market-data/stock/noii/bars`
- `/openapi/market-data/stock/noii/snapshot`
- `/openapi/market-data/stock/quotes`
- `/openapi/market-data/stock/snapshot`
- `/openapi/market-data/stock/tick`

**Streaming**

- `/openapi/market-data/streaming/subscribe`
- `/openapi/market-data/streaming/unsubscribe`

**Watchlists**

- `/openapi/market-data/watchlist/create`
- `/openapi/market-data/watchlist/delete`
- `/openapi/market-data/watchlist/instruments/add`
- `/openapi/market-data/watchlist/instruments/list`
- `/openapi/market-data/watchlist/instruments/remove`
- `/openapi/market-data/watchlist/instruments/update`
- `/openapi/market-data/watchlist/list`
- `/openapi/market-data/watchlist/update`

### Trading

**Activities**

- `/openapi/trade/activities/cash`

**Options**

- `/openapi/trade/option/order/cancel`
- `/openapi/trade/option/order/place`
- `/openapi/trade/option/order/preview`
- `/openapi/trade/option/order/replace`

**Orders**

- `/openapi/trade/order/batch-place`
- `/openapi/trade/order/cancel`
- `/openapi/trade/order/detail`
- `/openapi/trade/order/history`
- `/openapi/trade/order/open`
- `/openapi/trade/order/place`
- `/openapi/trade/order/preview`
- `/openapi/trade/order/replace`

**Stocks**

- `/openapi/trade/stock/order/cancel`
- `/openapi/trade/stock/order/place`
- `/openapi/trade/stock/order/preview`
- `/openapi/trade/stock/order/replace`

## Legacy unprefixed paths

Still present in the current SDK alongside the `/openapi/*` scheme — the SDK
ships both generations simultaneously. 17 paths:

- `/account/balance`
- `/account/positions`
- `/account/profile`
- `/app/subscriptions/list`
- `/instrument/corp-action`
- `/market-data/eod-bars`
- `/trade/calendar`
- `/trade/instrument`
- `/trade/instrument/tradable/list`
- `/trade/order/cancel`
- `/trade/order/detail`
- `/trade/order/place`
- `/trade/order/replace`
- `/trade/orders/list-open`
- `/trade/orders/list-today`
- `/trade/security`
- `/trading/orders/executions/list`

## Notable gaps between docs and SDK

Present in the reference documentation, absent from every SDK:

- `news` and `news-summary`
- The entire Connect API (OAuth 2.0 authorization, token exchange)
- The entire Broker API (account opening, ACH and wire funding, cash journals,
  agreements, document upload/download, and a dedicated event stream)
- FIX

Present in the SDK, not obviously mirrored in the reference index:

- `/openapi/config`, the runtime capability probe that reports whether token
  authentication is required

