# webull

[![CI](https://github.com/sfreiberg/webull/actions/workflows/ci.yml/badge.svg)](https://github.com/sfreiberg/webull/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/sfreiberg/webull/branch/main/graph/badge.svg)](https://codecov.io/gh/sfreiberg/webull/tree/main)
[![Go Reference](https://pkg.go.dev/badge/github.com/sfreiberg/webull.svg)](https://pkg.go.dev/github.com/sfreiberg/webull)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

An independent, open-source Go SDK for the [Webull OpenAPI](https://developer.webull.com/).

> **Status: pre-release.** Every endpoint the core product documents is
> implemented: trading (accounts, instrument data, orders across every
> asset class), the Market Data HTTP API (quotes, fundamentals, funds,
> screeners, watchlists), MQTT market-data streaming, real-time trade
> events over gRPC, access tokens, and the Connect API — verified against
> Webull's sandbox except where the
> [compatibility matrix](docs/COMPATIBILITY.md) says otherwise. The
> exceptions, with reasons, live in that matrix: news is blocked, and
> corporate actions turn out to belong to a separate Webull product that
> is excluded as a whole. The public API may change without notice until
> v1.0.0.

## Disclaimer

This is an independent open-source project. It is not affiliated with,
maintained by, authorized by, or endorsed by Webull. "Webull" and related marks
belong to their respective owners.

Trading involves risk. This software is provided without warranty of any kind.
You are responsible for any orders it places on your behalf.

## Documentation

- [API reference on pkg.go.dev](https://pkg.go.dev/github.com/sfreiberg/webull) — every exported type and method, with the behaviour verified against Webull's sandbox recorded on the items it affects
- [API compatibility matrix](docs/COMPATIBILITY.md) — what is implemented, what is planned, and what is out of scope
- [Discovery findings](docs/discovery/) — inventory of the Webull OpenAPI surface, authentication, streaming and wire format

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) for build,
test and code standards, and [SECURITY.md](SECURITY.md) for reporting
vulnerabilities.

## Installation

```
go get github.com/sfreiberg/webull
```

Go 1.26 or later.

## Authentication

Every request is signed with an app key and secret issued by the
[Webull developer portal](https://developer.webull.com/apis/docs/authentication/overview).
The SDK handles the signature (HMAC-SHA256 over a canonical form of the
request); you supply the credentials and choose an environment:

```go
client, err := webull.NewClient(webull.Config{
    AppKey:      os.Getenv("WEBULL_APP_KEY"),
    AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
    Environment: webull.Sandbox, // or webull.Production
})
```

Keep credentials out of source: read them from the environment or a secret
store. The SDK never logs them or includes them in error messages.

**Sandbox and production keys are separate.** A sandbox key returns
`404 Route Not Found` for every production host and vice versa — the most
common cause of a request that "should work" failing. There is no default
environment: choosing between simulated and real orders is not a decision the
SDK makes for you. Orders placed against `Production` are real.

Some deployments additionally require a per-user access token
(`token_check_enabled`); `client.TokenCheckEnabled(ctx)` reports whether yours
does. The sandbox does not, so most integrations need only the app key and
secret. When yours does, create a token with `client.CreateAccessToken(ctx)`
(it starts `PENDING` until verified through SMS and the Webull app; tokens
live 15 days and are not refreshable), check it with `CheckAccessToken`, and
supply it through `Config.AccessToken` — the SDK then sends it as
`x-access-token` on every request.

## Quick start

Credentials come from the [Webull developer portal](https://developer.webull.com/apis/docs/authentication/overview).
Sandbox and production keys are separate; a sandbox key returns `404 Route Not
Found` for every production path, which is worth knowing when a request fails.

```go
client, err := webull.NewClient(webull.Config{
    AppKey:      os.Getenv("WEBULL_APP_KEY"),
    AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
    Environment: webull.Sandbox, // or webull.Production; there is no default
})
if err != nil {
    log.Fatal(err)
}

accounts, err := client.Trade.Accounts(ctx)
if err != nil {
    log.Fatal(err)
}
acct := accounts[0]

balance, err := client.Trade.Balance(ctx, acct.AccountID)
```

### Buying shares

```go
order := &trade.Order{
    Symbol:     "AAPL",
    Side:       trade.Buy,
    Type:       trade.Limit,
    Quantity:   trade.Price("10"),
    LimitPrice: trade.Price("180.00"),
}

// Preview returns estimated cost and fees without placing anything.
preview, err := client.Trade.PreviewOrder(ctx, acct.AccountID, order)

receipt, err := client.Trade.PlaceOrder(ctx, acct.AccountID, order)
if err != nil {
    var apiErr *webull.APIError
    if errors.As(err, &apiErr) {
        log.Fatalf("rejected: %s (%s)", apiErr.Message, apiErr.Code)
    }
    // A transport failure means the outcome is unknown. The SDK generated
    // order.ClientOrderID before sending, so the order can be looked up
    // rather than blindly resent.
    log.Fatalf("outcome unknown for %s: %v", order.ClientOrderID, err)
}
fmt.Println("placed", receipt.OrderID)
```

The SDK fills in what Webull requires but a caller should not have to know:
market, combo type, entrust type, the trading session, and the client order ID.
It also validates locally — a limit order without a limit price fails before
any request is sent.

### Buying a call

Option legs are described by underlying, strike, expiry and type. `LegFromSymbol`
builds one from the OCC symbols that `OptionContracts` returns.

```go
chain, err := client.Trade.OptionContracts(ctx, trade.OptionContractsRequest{
    UnderlyingSymbols: []string{"AAPL"},
    OptionType:        trade.Call,
    StartDate:         "2026-12-18",
    EndDate:           "2026-12-18",
})

leg, err := trade.LegFromSymbol(chain.Contracts[0].Symbol) // e.g. AAPL261218C00240000

receipt, err := client.Trade.PlaceOrder(ctx, acct.AccountID, &trade.Order{
    Symbol:         "AAPL",
    Side:           trade.Buy,
    Type:           trade.Limit,
    Quantity:       trade.Price("1"),
    LimitPrice:     trade.Price("5.50"),
    PositionIntent: trade.BuyToOpen,
    Legs:           []trade.OrderLeg{leg},
})
```

### Attaching a take-profit and stop-loss

A bracket is one placement: the entry plus its exits, cancelled together.

```go
combo := trade.Bracket(
    &trade.Order{Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit, Quantity: trade.Price("10"), LimitPrice: trade.Price("180")},
    &trade.Order{Symbol: "AAPL", Side: trade.Sell, Type: trade.Limit, Quantity: trade.Price("10"), LimitPrice: trade.Price("195")},
    &trade.Order{Symbol: "AAPL", Side: trade.Sell, Type: trade.StopLoss, Quantity: trade.Price("10"), StopPrice: trade.Price("170")},
)
receipt, err := client.Trade.PlaceCombo(ctx, acct.AccountID, combo)
// later
err = client.Trade.CancelCombo(ctx, acct.AccountID, combo)
```

### A vertical spread

Multi-leg strategies are one `Order` with a strategy and its legs.

```go
long, _ := trade.LegFromSymbol("AAPL261218C00240000")
short, _ := trade.LegFromSymbol("AAPL261218C00250000")
long.Side, short.Side = trade.Buy, trade.Sell

receipt, err := client.Trade.PlaceOrder(ctx, acct.AccountID, &trade.Order{
    Symbol:         "AAPL",
    Side:           trade.Buy,
    Type:           trade.Limit,
    Quantity:       trade.Price("1"),    // spreads
    LimitPrice:     trade.Price("3.50"), // net debit
    OptionStrategy: trade.StrategyVertical,
    Legs:           []trade.OrderLeg{long, short},
})
```

### Other asset classes

Set `InstrumentType`; the rules for each asset class are checked locally.

```go
// Crypto, sized in coins (up to eight decimal places), $2 minimum.
client.Trade.PlaceOrder(ctx, cryptoAcct.AccountID, &trade.Order{
    InstrumentType: trade.InstrumentCrypto, Symbol: "BTCUSD",
    Side: trade.Buy, Type: trade.Limit, TimeInForce: trade.GTC,
    Quantity: trade.Price("0.001"), LimitPrice: trade.Price("60000"),
})

// Event contracts are limit-only and need the outcome being bought.
client.Trade.PlaceOrder(ctx, eventsAcct.AccountID, &trade.Order{
    InstrumentType: trade.InstrumentEvent, Symbol: "KXRATECUTCOUNT-26DEC31-T3",
    Side: trade.Buy, Type: trade.Limit,
    Quantity: trade.Price("5"), LimitPrice: trade.Price("0.10"),
    EventOutcome: trade.OutcomeYes,
})
```

Futures, crypto and event contracts each have their own account; use the one
whose `AccountClass` matches.

### Changing a working order

Only the fields you set are changed; the rest are left as they are.

```go
_, err = client.Trade.ReplaceOrder(ctx, acct.AccountID, trade.OrderModification{
    ClientOrderID: order.ClientOrderID,
    LimitPrice:    trade.Price("182.00"),
})
```

### Cancelling

```go
_, err = client.Trade.CancelOrder(ctx, acct.AccountID, order.ClientOrderID)
```

Cancel, replace and lookup are keyed by the client order ID, not Webull's own
`OrderID`. An `Order` value describes one placement: once accepted, its ID is
consumed, and placing it again returns `trade.ErrDuplicateOrder`.

## Market data

```go
snaps, err := client.MarketData.Snapshots(ctx, marketdata.SnapshotsRequest{
    Symbols: []string{"AAPL", "SPY"}, ExtendedHours: true,
})
fmt.Println(snaps[0].Price.Decimal, snaps[0].LastTradeTime)

bars, err := client.MarketData.Bars(ctx, marketdata.BarsRequest{
    Symbols: []string{"AAPL"}, Timespan: marketdata.Daily, Count: 30,
})
```

Data the key is not subscribed to fails with `marketdata.ErrNotSubscribed`, and
the wrapped `*webull.APIError` names the product required.

## Trade events

```go
stream, err := client.Events.Subscribe(ctx, events.SubscribeRequest{
    AccountIDs: []string{acct.AccountID},
})
defer stream.Close()
for {
    ev, err := stream.Recv()
    if err != nil {
        break
    }
    if ev.Kind == events.KindOrder && ev.Order != nil {
        fmt.Println(ev.Order.Scene, ev.Order.Symbol, ev.Order.FilledQuantity)
    }
}
```

`Subscribe` blocks until the server acknowledges, so a returned stream is
live. `Recv` swallows heartbeats and reconnects on transient failures;
authentication rejection and the server's connection limit surface as
`events.ErrAuthFailed` and `events.ErrConnectionLimit` instead of being
retried.

## Market data streaming

```go
stream, err := client.Streaming.Connect(ctx)
defer stream.Close()
err = stream.Subscribe(ctx, streaming.SubscribeRequest{
    Symbols: []string{"AAPL"},
    Types:   []streaming.SubType{streaming.SubSnapshot, streaming.SubTick},
})
for {
    msg, err := stream.Recv(ctx)
    if err != nil {
        break
    }
    if msg.Type == streaming.TypeTick && msg.Tick != nil {
        fmt.Println(msg.Tick.Symbol, msg.Tick.Price, msg.Tick.Volume)
    }
}
```

The MQTT connection carries the data; `Subscribe` and `Unsubscribe` are
signed HTTP calls that gate it, and a subscribe refused for a bad key is
`streaming.ErrSubscribeFailed`. A dropped connection reconnects and replays
its subscriptions.

## Connect API (OAuth)

To act on *another* user's Webull account — the model for a platform whose
users link their Webull accounts — use the `connect` package's OAuth flow:

```go
authorizer, _ := connect.NewAuthorizer(connect.Config{
    ClientID: clientID, ClientSecret: clientSecret,
    AppKey: appKey, AppSecret: appSecret,
    Environment: webull.Production,
    RedirectURI: "https://app.example.com/webull/callback",
})

// Send the user here; on the callback, exchange the code.
url := authorizer.AuthorizationURL(state)
token, _ := authorizer.ExchangeCode(ctx, code)

// Trade on the user's behalf; the access token refreshes itself. Use
// ClientFromStore to keep the rotating token pair in a store of your own.
client, _ := authorizer.Client(token)
accounts, _ := client.Trade.Accounts(ctx)
```

Connect credentials are issued by Webull to partner platforms, not
self-service, so this is implemented to the documentation and marked
`Unverified` in the [compatibility matrix](docs/COMPATIBILITY.md).

## Numbers

Every price, quantity and amount is a fixed-point decimal, never a float.
The types embed [shopspring/decimal](https://github.com/shopspring/decimal),
so all of its methods are available directly — `snapshot.Price.String()`,
`amount.Decimal.Equal(...)` and so on. Optional fields use a nullable form so
that "not reported" is distinguishable from zero. `trade.Price("1.50")` is a
shorthand for setting one.

The wrapper adds one thing over shopspring's own types: it decodes Webull's
absent forms — JSON `null` **and an empty string** — to an absent or zero
value. Webull returns `""` for an unreported number (an option greek outside
market hours, for instance), which shopspring's own decoder rejects.

## Errors

Failures from Webull are `*webull.APIError`, carrying the HTTP status, Webull's
error code and message, and a request ID for support. They match sentinel
errors with `errors.Is`:

```go
if errors.Is(err, webull.ErrRateLimited) {
    var apiErr *webull.APIError
    errors.As(err, &apiErr)
    time.Sleep(apiErr.RetryAfter)
}
```

The SDK never retries a `POST`: in this API a replayed order is a duplicated
order.

## Troubleshooting

A few failures are common enough to name, most of them settled while building
this SDK against the live sandbox:

| Symptom | Usual cause |
|---|---|
| `404 Route Not Found` | A sandbox key against production, or the reverse — the two are not interchangeable. Also returned for an unprovisioned key. |
| `webull: AppKey and AppSecret are required` from `NewClient` | Empty credentials — the SDK rejects them before any request, so this never reaches the server as a 401. |
| `417 OPENAPI_PARAM_ERR` | A parameter Webull rejected. Webull uses **417**, not 400, for validation errors; the message names the field. |
| `403 MARKET_DATA_NOT_SUBSCRIBED` | The key is not entitled to that market data. Matches `marketdata.ErrNotSubscribed`, and the message names the product (for example `STOCK QUOTES LV2`). |
| `429 TOO_MANY_REQUESTS` | Rate limited. Matches `webull.ErrRateLimited`; `APIError.RetryAfter` carries Webull's requested delay. |
| An order rejected off-hours | The sandbox enforces market hours for `DAY` orders. Use `GTC`, or trade during the regular session. |

Some endpoints are implemented but cannot be exercised by a sandbox key —
entitlement-gated data, or endpoints the sandbox serves empty. These are
marked `Unverified` in the [compatibility matrix](docs/COMPATIBILITY.md) with
the specific reason, so a surprising empty result can be checked against a
known limitation rather than mistaken for a bug.

## License

[MIT](LICENSE)
