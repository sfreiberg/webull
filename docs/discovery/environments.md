# Environments, hosts and regions

## US hosts

The SDK resolves hosts by region and API type. For the US:

| API type | Host | Serves |
|---|---|---|
| `api` | `api.webull.com` | Trading, accounts, positions, orders |
| `quotes-api` | `data-api.webull.com` | Listed by the SDK for market data; in fact the MQTT broker |
| `events-api` | `events-api.webull.com` | gRPC trade event streams |

The MQTT broker host is **not** in this table. The streaming client accepts it as
a parameter, and `/openapi/config` is the likely discovery mechanism. Unverified.

`us` is the SDK's default region.

## Sandbox

Confirmed against the live API with sandbox credentials.

| API type | Production | Sandbox |
|---|---|---|
| Trading / accounts | `api.webull.com` | `api.sandbox.webull.com` (**verified**) |
| gRPC events | `events-api.webull.com` | `events-api.sandbox.webull.com` |
| Market data HTTP | `api.webull.com` | `api.sandbox.webull.com` (**verified**) |
| MQTT streaming | `data-api.webull.com` | `data-api.sandbox.webull.com` (resolves; unverified) |

**Sandbox and production credentials are separate.** The same signed requests
that succeed against `api.sandbox.webull.com` return `404 Route Not Found` from
`api.webull.com` — every path, including ones that certainly exist there. The
gateway appears not to route at all for a key that is not provisioned for the
environment, so a production key is a separate issuance rather than the same key
with wider scope.

That 404-rather-than-401 behaviour is worth remembering when diagnosing: a 404
from a path known to be correct means the wrong environment or an unprovisioned
key, not a wrong URL.

`token_check_enabled` is **false** in sandbox, so the signature alone
authenticates and no access token is required. Whether production differs is
unverified.

Market data has a sandbox: the trading host serves live snapshots, ticks and
bars over HTTP in both environments. The `data-api` hosts that Webull's SDKs
route market data to resolve but hang on every HTTPS request; their DNS names
(`us-openapi-push…`, `…-pb-sandbox…`) mark them as the MQTT push brokers, and
the host table reserves them for streaming. The sandbox also simulates market
hours for order placement, rejecting DAY orders after the close.

## Connect API hosts

The Connect API uses its own hosts, which do not follow the `.sandbox` insertion
pattern above — production carries a `us-` prefix the sandbox host lacks. They
are tabulated in [connect-api.md](connect-api.md#hosts). Endpoint resolution must
treat them as table entries rather than deriving one from the other.

## Safety requirement

Integration tests must be *impossible* to accidentally point at production. Since the difference between environments is a hostname, this needs a
deliberate guard rather than a convention — for example, integration tests
refusing to run unless the resolved host matches the sandbox host, checked at the
transport layer rather than in each test.

This is worth building carefully. The failure mode is placing real orders against
a real account from a test run.

## Other regions

Webull operates twelve regional deployments sharing this API shape: US, HK, JP,
SG, TH, AU, MY, UK, BR, MX, ZA, EU.

Hosts follow a consistent pattern — `api.webull.com` becomes `api.webull.hk`,
`api.webull.co.jp`, `api.webull.com.au` and so on, with `data-api.` and
`events-api.` prefixes applied the same way. Not every region appears in every
mapping.

Scope for this SDK is **US only**, as agreed. The relevant design consequence is
that region should be modelled as configuration resolving to a host set, not
hardcoded, which endpoint overrides require anyway. Adding a region
later then means adding a table entry, not restructuring.

## Endpoint overrides

Per-endpoint overrides are needed for testing, proxies and mock servers. The
SDK's own resolver chain does this with a user-customised resolver layered over
the built-in table, which is a reasonable shape to mirror: a default table, an
optional override map, and a documented precedence between them.
