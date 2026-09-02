# Connect API

The Connect API is Webull's OAuth 2.0 product for third-party platforms acting
on behalf of *other people's* Webull accounts. Webull's documentation names
TradingView and SnapTrade as the archetype: a trading platform whose users hold
Webull brokerage accounts and want to trade them without leaving that platform.

It is not the path an individual takes to trade their own account. That is the
Trading API, authenticated with an app key and secret — what the rest of this
SDK is designed around.

## What it actually is

Not a separate API surface. Webull's documentation is explicit that the Connect
API and the Trading API have identical functionality across the Account, Assets
and Orders modules, and that the only difference is the base URL.

So Connect is an alternate front door: the same operations, reached at a
different host, authenticated differently. That has a large bearing on what it
costs us to support — see [Implementation cost](#implementation-cost) below.

## Authorization flow

Standard OAuth 2.0 authorization code:

1. The user chooses to connect their Webull account from the third-party
   application.
2. They are redirected to Webull's hosted login and authorization page.
3. Webull redirects back to the application's registered callback with an
   authorization code.
4. The application's server exchanges that code for an access token.
5. Subsequent Trading API calls are made with that token, on the user's behalf.

Observed constraints:

| Property | Value |
|---|---|
| Authorization code lifetime | 60 seconds, single use |
| Access token lifetime | 30 minutes; exact expiry returned in the response |
| Refresh | Refresh token, via a documented refresh endpoint |

The 60-second single-use code and 30-minute token are both short enough that
they need designing for rather than treating as edge cases. A token store that
refreshes lazily on 401 will thrash; refreshing proactively ahead of expiry is
the sounder approach.

## Hosts

| Environment | Purpose | Host |
|---|---|---|
| Production | Authorization, Account, Trading | `us-oauth-open-api.webull.com` |
| Sandbox | Authorization, Account, Trading | `oauth-open-api.sandbox.webull.com` |
| Production | Login redirect | `passport.webull.com/oauth2/authenticate/login` |
| Sandbox | Login redirect | `passport.webull.com/oauth2/sandbox/authenticate/login` |

The login redirect shares a hostname across environments and distinguishes them
by path; the API hosts are distinguished by hostname. The application does not
send the user to `passport.webull.com` itself: the official reference
(`developer.webull.com/apis/docs/reference/connect-api/get-authorization-code`)
gives the authorization URL as `GET /oauth2/auth-codes/get` on the API host,
which redirects the browser to the hosted login page above. Note also that the
production host carries a `us-` prefix that the sandbox host does not, so the
two are not related by a simple substitution the way the Trading API hosts are.
Endpoint resolution must treat these as table entries, not as a pattern.

## Credentials

Registration issues five values, not two:

| Credential | Purpose |
|---|---|
| `client_id` | Identifies the application in the OAuth flow |
| `client_secret` | Server-to-server authentication |
| `scope` | Granted access scope |
| `app_key` | Request signing |
| `app_secret` | Request signing |

**Connect requests are both signed and bearer-authenticated.** OAuth layers on
top of the existing HMAC signing scheme rather than replacing it. Any assumption
that a token makes signing unnecessary is wrong, and would produce requests that
fail in a confusing way.

## Access is partner-gated

Credentials are not self-service. Registration means contacting Webull's API team
at `connect.api@webull-us.com` with a company name and a redirect URL, and
waiting for manual issuance.

This is the same obstacle that excluded the Broker API: we cannot obtain
credentials, so we cannot verify our implementation against a live endpoint.

## Implementation cost

The reason Connect remains in scope where the Broker API did not is that the two
have very different cost profiles despite sharing the same testability problem.

The Broker API is roughly 115 endpoints with domain models — account opening,
funding, journals, agreements — that exist nowhere else in the SDK. Every one
would be new, unverified code.

Connect adds **no new endpoints at all**. It is the Trading API reached at a
different host with an additional credential. If the transport layer treats the
authentication strategy and base host as configurable — which it should for
testing and endpoint overrides regardless — then supporting Connect is the OAuth
authorization dance plus a client constructor, reusing models that the Trading
API work has already exercised and tested.

That difference is what makes shipping it defensible: a small, well-understood
surface marked `Unverified` is an honest position in a way that 115 untested
money-moving endpoints would not be.

## Design consequences

These apply from Phase 3, before any Connect code is written:

- **Authentication must be a strategy, not a hardcoded step.** The transport
  needs to accommodate signing alone, and signing plus a bearer token, without
  service packages knowing which is in play.
- **Base host must be per-client configuration**, not a package-level constant,
  so a `trade.Client` can be pointed at either the Trading or Connect host.
- **Token storage must be pluggable**, since a Connect integration holds tokens
  for many users rather than one. An in-memory default is right for the SDK;
  anything persistent belongs to the application.
- **Refresh must be concurrency-safe.** With 30-minute tokens and potentially
  many users, concurrent refresh of the same token is a realistic race rather
  than a theoretical one.

Getting these right in Phase 3 costs little. Retrofitting them after the Trading
API is written would mean reworking the transport layer.

## Unverified

Everything here comes from documentation. No official SDK in any language
implements the Connect API, so there is no reference implementation to check
behaviour against, and no credentials to test with.

Specifically unconfirmed: the scope value format, whether refresh tokens rotate
on use, whether token revocation is supported, and the error shape returned for
an expired or reused authorization code.
