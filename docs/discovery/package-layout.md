# Proposed package layout

§6 of the design proposal offers an illustrative tree and explicitly says the
final structure should follow real API boundaries. Now that those boundaries are
known, this is the concrete proposal.

## Layout

```
github.com/sfreiberg/webull

  webull                  client.go, config.go, environment.go, errors.go,
                          decimal.go, version.go
  trade                   accounts, positions, orders, options, futures,
                          crypto, event contracts, activities
  marketdata              quotes, snapshots, bars, ticks, depth, screeners,
                          instruments, fundamentals, watchlists
  events                  gRPC trade event streams
  streaming               MQTT real-time market data
  connect                 Connect API and OAuth 2.0

  internal/transport      HTTP plumbing, retry, response decoding
  internal/signing        canonical string construction and HMAC
  internal/wbproto        vendored .proto files and generated Go bindings
  internal/testutil       httptest and bufconn helpers, fixtures, fake clock

  examples/...            runnable programs per §32
```

## Why this shape

**`trade` and `marketdata` are the load-bearing split.** They are separate hosts
(`api.webull.com` and `data-api.webull.com`), separate rate-limit pools, and
separate entitlements. Users of one frequently do not use the other. Splitting
them mirrors how Webull actually deploys the API rather than how it documents it.

**`events` and `streaming` are separate packages, not one `stream` package.**
They share nothing operationally: different protocols (gRPC vs MQTT), different
hosts, different lifecycle, different failure modes. Merging them would create a
package whose two halves never touch. Naming them for what they carry — trade
events, market data streaming — is clearer than naming them for their transport.

**Market data is one package, not one per asset class.** Stocks, options,
futures, crypto and event contracts share request and response shapes almost
entirely; only the path segment and a few fields differ. Five packages would mean
five near-identical model sets. Asset class is better expressed as a typed
parameter than as a package boundary.

**Fundamentals and screeners live in `marketdata` rather than getting their own
packages.** They are on the same host with the same entitlement and the same
auth. A separate package would buy nothing but an extra import.

**`internal/wbproto` holds both the vendored `.proto` files and generated code.**
Keeping the schema next to its output makes the regeneration step obvious and
keeps the Apache-2.0 provenance of the definitions in one place. Generated code
is committed per §46 so ordinary users need no protobuf toolchain.

**No `broker` package yet.** Pending the scope decision in
[open-questions.md](open-questions.md#2). If it proceeds it belongs in its own
package, since its auth model and domain differ materially from individual
trading — exactly the condition §21 describes.

**No generic utility package.** §6 asks us to avoid these and there is no need:
signing belongs in `internal/signing`, transport concerns in
`internal/transport`, and the decimal type in the root package where its
documentation is discoverable.

## Root package contents

`Decimal` lives in the root rather than a subpackage because every service
package returns it, and a subpackage would create an import that appears in
almost every public signature for no benefit. See
[wire-format.md](wire-format.md).

The top-level `Client` composes the service clients per §8:

```go
type Client struct {
    Trade      *trade.Client
    MarketData *marketdata.Client
    Connect    *connect.Client
}
```

Streaming clients are deliberately **not** fields on `Client`. §8 requires that
long-lived connections be created explicitly rather than opened during client
construction, so `events.New(...)` and `streaming.New(...)` are constructed by
the caller and have `Close` methods. Hanging them off `Client` would imply they
share its lifetime, which they do not.

## Import direction

Service packages depend on `internal/*` and on the root package for shared types.
The root package must not import service packages, or `Client` composing them
creates a cycle. In practice this means shared types — `Decimal`, `APIError`,
`Environment`, the config struct — live in the root, and the root's `Client`
constructor is the one place that reaches into service packages.

This is worth stating now because it is the constraint most likely to be violated
accidentally in Phase 4, and unwinding a cycle later is disruptive.
