# Authentication and request signing

Webull authenticates every request with an HMAC signature over a canonical
string, using an app key and app secret issued from the account management page.
Some deployments additionally require a bearer access token obtained through a
token endpoint. There is no OAuth in the individual-developer flow; OAuth appears
only in the Connect API, which no SDK implements.

The algorithm below was reconstructed from three independent official
implementations — Python, Java, and Webull's own Go CLI — which agree with each
other. It is described here so that our implementation can be written from the
description rather than transcribed from Apache-2.0 source.

## Signature headers

Every signed request carries:

| Header | Value |
|---|---|
| `x-app-key` | The app key |
| `x-timestamp` | Request time, ISO-8601 UTC, second precision (`2006-01-02T15:04:05Z`) |
| `x-signature-version` | `1.0` |
| `x-signature-algorithm` | `HMAC-SHA256` (see note on SHA-1 below) |
| `x-signature-nonce` | A fresh UUID per request |
| `x-signature` | The computed signature |

Not sent as a header but participating in the signature: the request `Host`.

Other headers seen in use: `X-Request-Id` for correlation, `x-access-token` when
token authentication is enabled, and `x-version` for the API version. Two more,
`x-webull-client-source` and `wb-user-id`, appear in the SDK's header constants
with no documented meaning.

## Building the string to sign

1. **Collect the signing parameters.** Take the six signature headers above plus
   `Host`, and lowercase every key. `Host` becomes `host`. This lowercasing is
   not cosmetic — it determines sort order in step 3.

2. **Merge in the query string.** For each query parameter, add it to the same
   map. If a key already exists, join the existing and new values with `&` rather
   than overwriting. (This collision path is exercised by the official
   implementations but is unusual in practice.)

3. **Sort by key, ascending, bytewise.** Render each entry as `key=value` and
   join the pairs with `&`.

4. **Prepend the path.** If the request has a URI path, the string becomes
   `path` + `&` + the joined pairs. If it does not, it is just the joined pairs.

5. **Append the body digest.** If there is a request body, take its SHA-256,
   hex-encode it, **uppercase it**, and append `&` + that digest. The body is
   hashed in its exact serialized form, so the JSON encoding must be byte-stable
   between hashing and transmission — this is the single easiest thing to get
   wrong.

6. **Percent-encode the whole string.** Every character outside
   `A-Za-z0-9-._~` is encoded, *including* `/`. Space encodes as `%20`, not `+`.
   In Go, `url.QueryEscape` followed by replacing `+` with `%20` produces this.

## Computing the signature

The signing key is the **app secret with a single `&` appended**. Compute
HMAC-SHA256 over the encoded string with that key, then base64-encode the raw
digest. That value is `x-signature`.

## A trap: the official clients disagree on the default

The Python SDK ships three algorithm modules, reporting themselves as
`HMAC-SHA1`, `SHA256withRSA` and `HMAC-SHA256`. Which one is used is not,
however, a choice: the signature composer overwrites whatever the caller
selected with the `HMAC-SHA256` implementation unconditionally, before the
algorithm header is set. The MD5 body-digest branch sitting beside it is
unreachable as a result.

Webull's Go CLI does the opposite. It defaults to **HMAC-SHA1** when no
algorithm is given, and in that mode digests the body with MD5 rather than
SHA-256.

So two first-party clients ship opposite defaults. Ours will:

- **Sign with HMAC-SHA256 and digest the body with SHA-256.** This follows the
  maintained Python client, which is what current traffic actually looks like.
- **Not expose an algorithm toggle at all.** An SDK that can be configured into
  HMAC-SHA1 with an MD5 digest is a liability rather than a feature, and the Go
  CLI demonstrates how easily that default ships by accident.

It follows that whether the server still accepts HMAC-SHA1 is not worth
establishing. We will never send it, so the answer cannot change the
implementation. What matters is only that our SHA-256 signature is accepted,
which the first successful authenticated request demonstrates.

## Token lifecycle

Token authentication is conditional, which is unusual and worth designing for
explicitly.

1. The client calls `GET /openapi/config` at startup.
2. The response contains `token_check_enabled`.
3. If false, signature headers alone authenticate every request.
4. If true, the client must additionally obtain an access token and send it as
   `x-access-token`.

Token endpoints, in the SDK's path scheme:

| Operation | Path |
|---|---|
| Create | `POST /openapi/auth/token/create` |
| Refresh | `POST /openapi/auth/token/refresh` |
| Check | `POST /openapi/auth/token/check` |

The documentation uses a different scheme for the same operations
(`/auth/tokens/create`) and additionally documents a separate **client token**
pair, `/auth/client-tokens/create` and `/auth/client-tokens/refresh`, rate-limited
far more generously at 600 per 60 seconds versus 10 per 10 seconds. The client
token appears to be a market-data-specific credential. No SDK implements it. See [open-questions.md](open-questions.md#1).

The Python SDK persists tokens to a file in a configurable directory. We should
not replicate that as a default. Token storage should be pluggable, and writing
credentials to disk without being asked is a poor default for a library.
An in-memory store should be the default, with a file-backed implementation
available for examples and CLI use.

## Implications for our design

- Signing must be centralised and independently unit-testable with a fixed clock
  and fixed nonce. The canonical-string builder should be a pure
  function of (method, path, query, headers, body) so it can be tested against
  known vectors without a network.
- Because the body digest covers the exact serialized bytes, the signer must
  receive the marshalled body, not the struct. Marshalling twice risks a
  mismatch if map ordering ever differs.
- Token refresh must be concurrency-safe, and the conditional nature of token
  checking means the client needs a startup probe or lazy equivalent. Fetching
  `/openapi/config` eagerly in `NewClient` would make construction perform I/O.
  Resolving it lazily on first request is preferable — constructing a client
  should not require a network round trip.
- Secrets must never reach logs or error strings. The signing key is derived from
  the app secret, so error messages from the signer must never include the
  computed key or the string to sign.
