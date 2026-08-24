# Environments, hosts and regions

## US hosts

The SDK resolves hosts by region and API type. For the US:

| API type | Host | Serves |
|---|---|---|
| `api` | `api.webull.com` | Trading, accounts, positions, orders |
| `quotes-api` | `data-api.webull.com` | Market data over HTTP |
| `events-api` | `events-api.webull.com` | gRPC trade event streams |

The MQTT broker host is **not** in this table. The streaming client accepts it as
a parameter, and `/openapi/config` is the likely discovery mechanism. Unverified.

`us` is the SDK's default region.

## Sandbox

Sandbox hosts are published in the getting-started guides, though not in any SDK
endpoint table:

| API type | Production | Sandbox |
|---|---|---|
| Trading / accounts | `api.webull.com` | `api.sandbox.webull.com` |
| gRPC events | `events-api.webull.com` | `events-api.sandbox.webull.com` |
| Market data HTTP | `data-api.webull.com` | **unconfirmed** |
| MQTT streaming | `data-api.webull.com` | **not published** |

The pattern is to insert `.sandbox` before `.webull.com`, so
`data-api.sandbox.webull.com` is the obvious guess — but it is a guess, and the
market-data getting-started page muddies it further by listing the trading hosts
(`api.webull.com` / `api.sandbox.webull.com`) as its "API Endpoints" while the
SDK routes HTTP market data to `data-api.webull.com`. That is a second
docs-versus-SDK discrepancy of the same family as the path-scheme one.

For streaming the documentation names only a **Production MQTT** host. No sandbox
equivalent is given anywhere. Real-time market data may simply not have a
sandbox, which would be consistent with market data being an entitlement rather
than a simulated account.

Still unresolved:

1. Whether `data-api.sandbox.webull.com` exists.
2. Whether MQTT streaming has any sandbox at all.
3. Whether sandbox credentials are issued separately from production.
4. Whether sandbox simulates market hours. This determines how much market-hours
   machinery the test harness needs — possibly none.

The first two are answerable with a DNS lookup and one connection attempt once
we have credentials; the rate-limits page listing separate sandbox quotas for
every market-data endpoint is weak evidence that a sandbox does exist for them.

## Safety requirement

§37 requires that integration tests be *impossible* to accidentally point at
production. Since the difference between environments is a hostname, this needs a
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
hardcoded — which §9 requires anyway for endpoint overrides. Adding a region
later then means adding a table entry, not restructuring.

## Endpoint overrides

§9 requires per-endpoint overrides for testing, proxies and mock servers. The
SDK's own resolver chain does this with a user-customised resolver layered over
the built-in table, which is a reasonable shape to mirror: a default table, an
optional override map, and a documented precedence between them.
