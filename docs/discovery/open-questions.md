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
the nominal source of truth. One plausible
reading is that the documented scheme is a gateway that the SDKs have not
migrated to; another is that the docs describe a forthcoming version. Nothing in
either source states which.

**Resolving it requires credentials** — one signed request against each scheme
tells us which the server honours, and whether both do.

**Recommendation:** treat this as the first item in the sandbox validation
backlog and make it a gate on Phase 3. Until then, design the transport layer so
the path scheme is a single centralised mapping rather than string literals
scattered across service methods, which is good practice regardless. That way the answer
changes one file instead of eighty. Do not begin implementing trading endpoints
until this is settled — building against the wrong generation would waste most of
Phases 4 through 6.

## 2. Connect API and Broker API exist in the docs but not in any SDK

*Broker API resolved: excluded. Connect API remains in scope.*

Both are documented — Connect API has 4 reference pages plus OAuth 2.0 guides,
Broker API has 78 under `broker-fd-api` and a further 37 for broker market data
— but neither appears anywhere in the Python, Java, or Go references. There is no
OAuth code, no authorization-code exchange, no `redirect_uri` handling in any
official SDK.

**The Broker API is out of scope** by project decision. See
[../COMPATIBILITY.md](../COMPATIBILITY.md#broker-api--excluded) for the reasoning.

**The Connect API stays in scope** (Phase 7). Its credentials are partner-gated
— issued manually by Webull to registered companies — so like the Broker API we
almost certainly cannot test it. The reason it survives where the Broker API did
not is cost: Connect introduces no new endpoints, only a different host and an
extra credential over the Trading API we will already have built and tested. See
[connect-api.md](connect-api.md) for the full findings and for the design
consequences that land in Phase 3.

## 3. FIX protocol

*Resolved: excluded.*

`fix/about-fix`, `fix/fix-spec` and `fix/faq` document a FIX interface. It was
not part of the SDK's original scope and was surfaced by this inventory.

FIX (Financial Information eXchange) is a decades-old messaging protocol used
between institutions for order routing and execution reports. It is a persistent
TCP session with sequence-numbered messages, its own heartbeat and gap-fill
recovery semantics, and a tag-value wire encoding — architecturally unrelated to
the HTTP, gRPC and MQTT interfaces this SDK targets. Firms that use FIX normally
run a dedicated engine such as QuickFIX rather than getting it from a vendor SDK.

Excluded. It shares essentially no implementation with the rest of the SDK and
would roughly double the project's surface for an audience already served by
existing FIX libraries. Recorded in the compatibility matrix as a deliberate
exclusion so it does not resurface as an apparent gap during the final
completeness audit.

## 4. Sandbox — hosts found, coverage still uncertain

*Partially resolved.*

Sandbox is a firm project requirement, and the trading and event hosts are
published in the getting-started guides:

- `api.sandbox.webull.com`
- `events-api.sandbox.webull.com`

What remains unknown is whether market data has a sandbox at all. No
`data-api.sandbox.webull.com` is published, and the streaming guide names only a
production MQTT host. See
[environments.md](environments.md#sandbox) for the detail.

This matters for the test harness: if market data is production-only, then
market-data integration tests either hit production read-only endpoints or do not
run at all, and that is a design decision rather than an oversight to discover
later.

Remaining items are in the sandbox validation backlog in the compatibility
matrix.

## 5. Licensing

All five official repositories are Apache-2.0. Apache-2.0 and MIT are compatible
in the direction we need: we may read Apache-2.0 code for understanding and
publish original work under MIT. What we must not do is copy Apache-2.0 source
into this repository, because NOTICE-bearing Apache works carry attribution
obligations that MIT does not express.

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
  tiers with overlapping paths and different rate limits, which the SDKs do not
  model. The plan is to surface an entitlement failure as its own typed error
  rather than letting a bare 403 surface, so a caller can tell "your key lacks
  this tier" from "your request was wrong". Exact behaviour is unverified and is
  in the validation backlog.
- **`x-webull-client-source` and `wb-user-id` headers** exist in the SDK header
  constants with no documented explanation.
