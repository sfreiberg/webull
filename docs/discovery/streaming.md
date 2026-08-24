# Streaming: gRPC events and MQTT market data

Webull uses two streaming protocols for two different purposes. Order and
account events arrive over gRPC; real-time market data arrives over MQTT.

## Protobuf definitions are published

This was the main gating question for the project, because §18 of the design
proposal makes gRPC support conditional on obtaining protocol definitions, and
§3 allows an explicit completeness exception if they cannot be had.

**They can be had.** `.proto` source files ship in the official SDK repositories
under Apache-2.0:

| File | Repository | Purpose |
|---|---|---|
| `events.proto` | Python and Java SDKs | Trade event gRPC service |
| `message.proto` | Python and Java SDKs | MQTT market-data payloads |
| `quotes.proto`, `gateway.proto`, `api.proto` | Java SDK | Previous-generation gRPC quotes |
| `quote.proto`, `gateway.proto` | Previous-generation Python SDK | Previous-generation gRPC quotes |

**No completeness exception is needed for gRPC.** We can generate Go bindings
with `protoc-gen-go` and `protoc-gen-go-grpc` from the same definitions the
official SDKs use.

Per §46, generated code should be committed so ordinary users can build without
a protobuf toolchain, with regeneration documented. The `.proto` files themselves
should be vendored into the repository with their Apache-2.0 provenance recorded,
since they are the schema source and pinning them protects us from upstream drift.

## gRPC: trade events

`events.proto` is small — the entire service is one streaming RPC:

```proto
package grpc.trade.event;

service EventService {
  rpc Subscribe(SubscribeRequest) returns (stream SubscribeResponse) {}
}
```

The full method name on the wire is `/grpc.trade.event.EventService/Subscribe`.
Host for the US region is `events-api.webull.com`.

The notable design point is that **the payload is not typed by protobuf**.
`SubscribeResponse` carries `contentType` and `payload`, both `string` — the
actual event body is JSON carried inside a protobuf string field. Protobuf here
is an envelope, not a schema.

`EventType` enumerates the control plane rather than business events:
`SubscribeSuccess`, `Ping`, `AuthError`, `NumOfConnExceed`, `SubscribeExpired`.

Consequences for our design:

- Typed Go event structs must be defined by us against the JSON payloads, since
  protobuf does not describe them. §18's requirement that users not have to touch
  generated protobuf messages is therefore easy to satisfy at the envelope level
  and unavoidable work at the payload level.
- `Ping` must be handled to keep the stream alive.
- `NumOfConnExceed` and `SubscribeExpired` are distinct failure modes needing
  distinct typed errors. Blindly reconnecting on `NumOfConnExceed` would make the
  problem worse, so the reconnect policy must discriminate by event type rather
  than reconnecting on any disconnect.
- `AuthError` should not be retried at all.

The `SubscribeRequest` accepts a repeated `accounts` field, so one stream covers
multiple accounts.

## MQTT: real-time market data

The Python SDK builds on `paho.mqtt`, connecting on port **1883** with TLS
enabled and a TCP transport. Payloads are protobuf-encoded per `message.proto`.

Subscription is not purely an MQTT concern — there are HTTP endpoints
`/openapi/market-data/streaming/subscribe` and `.../unsubscribe` that register
interest, alongside the MQTT connection that delivers data.

Topics are structured as `instrumentId-dataType-interval`. Subscribe types are a
small enum: `QUOTE` (0), `SNAPSHOT` (1), `TICK` (2).

`message.proto` defines the payload types: `Basic`, `Snapshot`, `Quote`, `Tick`,
`AskBid`, `Order`, `Broker`, and event-contract variants. `AskBid` carries nested
`Order` (with `mpid`) and `Broker` entries, so full depth-of-book with market
participant identifiers is available.

Consequences for our design:

- §19 requires that ordinary users never construct topic strings. Since the topic
  format is a simple triple, a typed subscription request that renders the topic
  internally is straightforward.
- The MQTT broker hostname is not in the SDK's endpoint table and appears to be
  discovered at runtime — the client accepts an explicit `mqtt_host` and the
  `/openapi/config` endpoint is the likely source. **Unverified**; needs
  credentials to confirm.
- The previous-generation SDK had a `/market-data/streaming/token` endpoint,
  suggesting streaming may need its own credential. Whether that survives into
  the current generation is unverified.
- Paho's Go client (`eclipse/paho.mqtt.golang`) is the mature equivalent and
  matches §47's preference for not writing a protocol stack.

## Previous-generation gRPC quotes

`quotes.proto` / `gateway.proto` describe a gRPC market-data path
(`/openapi.Quote/StreamRequest`) present in the older SDKs. The current SDK
delivers market data over MQTT instead.

This looks like a superseded transport. It should not be implemented unless
Webull's documentation still presents it as current — worth one check during
Phase 9 rather than assuming either way.

## Testing

§36 requires local protocol tests. Both are tractable:

- **gRPC** — a real in-process server on a bufconn listener implementing
  `EventService`, which exercises our client against the actual generated stubs
  rather than a mock.
- **MQTT** — the seam should be an interface over the paho client so tests can
  drive connect, subscribe, publish, malformed payloads, and disconnect without a
  broker. A real broker in CI is possible but slower and flakier; an interface
  seam covers the reconnect and resubscribe logic that actually needs testing.

Neither requires market hours, since both are driven by fixtures. Only verifying
that live ticks actually flow requires an open market.
