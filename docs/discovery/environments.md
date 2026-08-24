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

A sandbox exists, but we do not yet know how to reach it.

The rate-limits documentation lists **separate Sandbox and Production quotas for
every endpoint** — frequently different ones, with market-data endpoints allowing
30 requests per 60 seconds in sandbox against 60 in production. An environment
with its own published quotas is not hypothetical.

But no official SDK exposes it. The endpoint table contains production hosts
only, there is no environment switch in any client constructor, and the only
trace of a non-production environment anywhere is a comment in a
previous-generation demo noting that a UAT domain can be set by overriding the
host manually.

So sandbox access appears to work by pointing the client at a different host,
with that host documented somewhere we have not yet found — or issued alongside
sandbox credentials.

**Unresolved, and it matters.** The design proposal makes sandbox central: §9
requires environment support, §37 requires opt-in sandbox integration tests, and
Milestones 2 through 8 all carry sandbox verification in their acceptance
criteria. The testing arrangement agreed for this project assumes it too.

Three things go in the sandbox validation backlog:

1. The sandbox hostnames for all three API types plus MQTT.
2. Whether sandbox credentials are separate from production credentials.
3. Whether sandbox simulates market hours, or accepts orders and streams data
   around the clock. This determines how much market-hours machinery the test
   harness actually needs — possibly none.

Until this is settled, `Environment` should still exist in the config API as
agreed, with `Production` implemented and `Sandbox` resolving to endpoints we fill
in once known. That keeps the public API stable across the discovery.

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
