# Proposed package layout

Package structure should follow real API boundaries rather than mirroring how
Webull documents the API or how the Python and Java SDKs are organised. Now that
those boundaries are known, this is the concrete proposal.

## Layout

The root package sits at the repository root, so the import path is
`github.com/sfreiberg/webull` rather than `github.com/sfreiberg/webull/webull`.

```
webull/                     module github.com/sfreiberg/webull
├── go.mod
├── client.go               package webull — top-level client
├── config.go               credentials, options
├── environment.go          production/sandbox host resolution
├── errors.go               APIError and classification helpers
├── version.go              SDK version and User-Agent
│
├── trade/                  accounts, positions, orders, options, futures,
│                           crypto, event contracts, activities
├── marketdata/             quotes, snapshots, bars, ticks, depth, screeners,
│                           instruments, fundamentals, watchlists
├── events/                 gRPC trade event streams
├── streaming/              MQTT real-time market data
├── connect/                Connect API and OAuth 2.0
│
├── internal/
│   ├── transport/          HTTP plumbing, retry, response decoding
│   ├── signing/            canonical string construction and HMAC
│   ├── wbproto/            vendored .proto files and generated Go bindings
│   ├── query/            query-string builder shared by service packages

│   └── testutil/           httptest and bufconn helpers, fixtures, fake clock
│
└── examples/               runnable example programs
```

Resulting import paths:

```go
import (
    "github.com/sfreiberg/webull"
    "github.com/sfreiberg/webull/trade"
    "github.com/sfreiberg/webull/marketdata"
)
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
is committed so ordinary users need no protobuf toolchain.

**No `broker` package.** The Broker API is out of scope; see
[../COMPATIBILITY.md](../COMPATIBILITY.md#broker-api--excluded). Were it ever
added it would belong in its own package, since its authentication model and
domain differ materially from individual trading — which is exactly why
excluding it costs nothing structurally.

**No generic utility package.** These accrete unrelated code and have no clear
owner, and there is no need for one here:
signing belongs in `internal/signing` and transport concerns in
`internal/transport`.

## Root package contents

Monetary and quantity values use `github.com/shopspring/decimal` directly rather
than a wrapper type of ours — see [wire-format.md](wire-format.md). That is the
SDK's one non-obvious public dependency, and it appears in service package
signatures rather than the root.

The top-level `Client` composes the service clients:

```go
type Client struct {
    Trade      *trade.Client
    MarketData *marketdata.Client
    Connect    *connect.Client
}
```

Streaming clients are deliberately **not** fields on `Client`. Long-lived
connections should be created explicitly rather than opened during client
construction, so `events.New(...)` and `streaming.New(...)` are constructed by
the caller and have `Close` methods. Hanging them off `Client` would imply they
share its lifetime, which they do not.

## Import direction

The root package imports the service packages in order to compose them, so
the service packages must not import the root package or there is a cycle.

Service packages therefore depend only on `internal/*` and on third-party
types such as `decimal.Decimal`. Anything a service package needs from the
root — the resolved host, the signing transport, the error decoder — is handed
to it at construction rather than imported. Errors surface as `*webull.APIError`
without `trade` naming that type, because the decoder that builds them is
injected into the transport by the root package.

The practical consequence is that `trade.New` takes an `internal/transport`
type. That is deliberate: it makes the constructor unusable outside this module,
which is the point — `webull.NewClient` is the supported way to obtain one.
