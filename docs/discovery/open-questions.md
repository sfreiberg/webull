# Open questions and decisions needed

Ordered by how much they block implementation.

## 1. The documentation and the SDKs describe different API generations

**This is the most consequential finding of Phase 1 and it needs a decision
before Phase 3.**

The official reference documentation and the official SDKs do not agree on
endpoint paths. They are not variations in spelling; they are different schemes.

| Operation | Documentation | Current Python/Java SDK |
|---|---|---|
| List accounts | `GET /trading/accounts/list` | `/openapi/account/list` |
| Stock snapshot | `GET /market-data/stocks/snapshots/list` | `/openapi/market-data/stock/snapshot` |
| Create token | `POST /auth/tokens/create` | `/openapi/auth/token/create` |
| Top actives | `GET /market-data/screeners/top-actives/list` | `/openapi/market-data/screener/top-active` |

The documented scheme is consistently RESTful — plural collection nouns, an
explicit `/list` or `/get` verb suffix, no `/openapi` prefix. The SDK scheme is
singular and prefixed.

Counting generations found across all sources, there are at least three and
arguably four:

1. **v1, unprefixed** — `/trade/order/place`, `/account/balance`. Still present
   in the current SDK.
2. **v2, `/openapi/account/orders/*`** — only in the previous-generation SDK.
3. **v3, `/openapi/trade/*`** — what the current SDK actually calls.
4. **The documented scheme** — `/trading/*`, `/market-data/*`, `/auth/*`.

Note that the current SDK ships *all* of generations 1 and 3 simultaneously, with
`v2`/`v3` subpackages layering over the same operations.

**Why this can't be settled from the desk:** both sources are official and
current. The Python SDK was updated the same week as this research; the docs are
the stated source of truth under §4 of the design proposal. One plausible
reading is that the documented scheme is a gateway that the SDKs have not
migrated to; another is that the docs describe a forthcoming version. Nothing in
either source states which.

**Resolving it requires credentials** — one signed request against each scheme
tells us which the server honours, and whether both do.

**Recommendation:** treat this as the first item in the sandbox validation
backlog and make it a gate on Phase 3. Until then, design the transport layer so
the path scheme is a single centralised mapping rather than string literals
scattered across service methods, which §45 asks for anyway. That way the answer
changes one file instead of eighty. Do not begin implementing trading endpoints
until this is settled — building against the wrong generation would waste most of
Phases 4 through 6.

## 2. Connect API and Broker API exist in the docs but not in any SDK

Both are documented — Connect API has 4 reference pages plus OAuth 2.0 guides,
Broker API has 78 under `broker-fd-api` and a further 37 for broker market data
— but neither appears anywhere in the Python, Java, or Go references. There is no
OAuth code, no authorization-code exchange, no `redirect_uri` handling in any
official SDK.

Consequences:

- The Connect API (Phase 7) and Broker API (Phase 10) must be implemented from
  documentation alone, with no reference implementation to check behaviour
  against. That is materially riskier than the Trading and Market Data work.
- The Broker API is far larger than the proposal assumed. It covers account
  opening, ACH and wire funding, cash journals, agreements, document upload and
  download, and its own event stream — 115 reference pages in total, which is
  more than the entire individual Trading and Market Data surface combined.
- Broker API access requires an enterprise relationship with Webull. We will not
  be able to test any of it.

**Recommendation:** keep Connect API in scope; it is a bounded OAuth 2.0 flow and
individual developers can plausibly use it. **Re-scope the Broker API**: I would
not treat 115 untestable endpoints as a v1.0 requirement. Options are to defer it
past v1.0, implement it as a documented best-effort with an explicit "unverified"
warning, or record it as a §3 completeness exception on the grounds that the
required credential class is unavailable to public developers. This is your call
and it materially changes the project's size.

## 3. FIX protocol is documented and absent from the proposal

`fix/about-fix`, `fix/fix-spec` and `fix/faq` document a FIX interface. The
design proposal never mentions FIX.

A FIX engine is not a small addition to an HTTP/gRPC/MQTT SDK — it is a separate
protocol stack with its own session semantics, sequence-number recovery and
persistence requirements, conventionally handled by a dedicated library.

**Recommendation:** declare FIX explicitly out of scope in the README and record
it in the compatibility matrix as a deliberate exclusion rather than an
oversight. Reversing that later is a clean addition; discovering it silently
missing at Milestone 11 is not.

## 4. Sandbox exists, but its base URL is undocumented in the SDKs

The rate-limits page lists separate Sandbox and Production quotas for every
endpoint, so a sandbox environment demonstrably exists. But the SDK endpoint
table contains only production hosts, and no SDK exposes a sandbox toggle.

We do not yet know the sandbox hostnames, whether sandbox credentials are issued
separately, or whether sandbox simulates market hours. All three matter for the
test harness design agreed earlier.

**Recommendation:** sandbox host discovery goes in the validation backlog and is
the first thing to check once credentials exist.

## 5. Licensing

All five official repositories are Apache-2.0. Apache-2.0 and MIT are compatible
in the direction we need: we may read Apache-2.0 code for understanding and
publish original work under MIT. What we must not do is copy Apache-2.0 source
into this repository, because §4 of NOTICE-bearing Apache works carries
attribution obligations that MIT does not express.

Two things follow:

- Protocol constants — endpoint paths, header names, the signing algorithm's
  steps, protobuf field numbers — are facts about a wire protocol, not creative
  expression, and being original about them would simply mean being wrong. These
  match.
- Structure, naming, abstractions, and documentation prose are written fresh.

One item needs care: the Python SDK's signing code carries an Apache-2.0 header
and states it was adapted from Alibaba's `aliyun-openapi-python-sdk`. Our signing
implementation should be written from the algorithm description in
[authentication.md](authentication.md), not transcribed from either source.

## 6. Smaller items

- **`news` and `news-summary`** appear in the reference docs but in no SDK. Their
  request and response shapes are unverified.
- **Display vs Non-Display market data** are separate documented entitlement
  tiers with overlapping paths and different rate limits. The SDK does not model
  the distinction. We need to decide whether the Go API exposes it or hides it —
  it affects which endpoints a given key can call, so surfacing it as a typed
  entitlement error is probably better than silently 403ing.
- **`x-webull-client-source` and `wb-user-id` headers** exist in the SDK header
  constants with no documented explanation.
