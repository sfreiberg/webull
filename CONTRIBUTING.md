# Contributing

Thanks for your interest. This document covers how the project is built and the
standards a change is held to.

This is an independent open-source project and is not affiliated with or
endorsed by Webull.

## Getting started

```
git clone https://github.com/sfreiberg/webull
cd webull
go test ./...
```

Go 1.27 and 1.26 are both supported and both are tested in CI. `go.mod`
declares 1.26, so that is the minimum.

## Running what CI runs

Before opening a pull request:

```
gofmt -l .                                  # must print nothing
go vet ./...
go test -race ./...
go test -covermode=atomic -coverprofile=coverage.txt ./...
go tool cover -func=coverage.txt | tail -1
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1 run ./...
```

CI additionally runs `govulncheck` and `gitleaks`.

## Testing

**Coverage must not fall below 90%, and that is a floor rather than a target.**
The build fails below 90%; the intent is to stay close to 100%. New code should
arrive with tests, not acquire them later. Codecov holds changed lines to a
higher bar than the codebase average, because code written today has no legacy
excuse.

Coverage alone is not sufficient. Critical paths — request signing, order
construction, reconnect logic, token refresh — need tests that assert on
behaviour, not tests that merely execute the lines.

Tests come in two tiers:

**Unit and protocol tests** run always, everywhere, with no credentials, no
network and no dependency on market hours. They must be deterministic at 3am on
a Sunday. Use `httptest` for HTTP, an in-process listener for gRPC, and an
interface seam for MQTT. Where behaviour depends on time, inject a clock rather
than sleeping.

**Integration tests** run against Webull's sandbox and require credentials. They
run automatically when credentials are present and skip with an explicit,
reported reason when they are not. A skip must say why:

```go
t.Skip("integration: WEBULL_APP_KEY not set")
```

Never let an integration test pass silently when it did not run. A green suite
that ran nothing is worse than a red one.

Integration tests must be impossible to point at production by accident. This is
enforced in the client, not by convention — the failure mode is placing real
orders against a real account.

## Code standards

The public API is the product. It should feel like Go, not like a translation of
Webull's Python or Java SDKs.

- `context.Context` is the first parameter of every network operation.
- Typed requests and responses. No `any`, no `map[string]any` where a schema
  exists.
- Enumerations are string-based named types with declared constants, so an
  unrecognised value from Webull round-trips instead of failing to parse.
- Errors wrap with `%w` and are inspectable with `errors.Is` and `errors.As`.
- Monetary and quantity values use `github.com/shopspring/decimal`, never
  `float64`. See [docs/discovery/wire-format.md](docs/discovery/wire-format.md).
- Optional decimal fields are `decimal.NullDecimal` in both directions, with
  the `omitzero` tag on request fields, so absent is distinguishable from zero
  and unset fields are omitted rather than sent as `null`.
- No panics for recoverable errors. No package-level mutable state.
- Anything holding a connection or a goroutine exposes `Close`, and every
  goroutine has a defined path to exit.
- Public clients are safe for concurrent use, and say so in their doc comment.

### Security

- Never log credentials, tokens, authorization codes or signing material.
  Debug output must redact them, and redaction needs a test.
- Never include secrets in error strings. Errors from the signer must not
  contain the signing key or the string being signed.
- Examples read credentials from the environment. No credential ever appears in
  the repository, including in a test fixture.

### Dependencies

Every dependency needs a reason. Prefer the standard library. The current
non-standard dependencies are the gRPC and protobuf runtimes, an MQTT client,
and `shopspring/decimal` — each carrying a cost we judged worth paying rather
than reimplementing.

## Documentation

Every exported symbol has a doc comment. Package documentation explains what the
package covers, how clients are constructed, concurrency behaviour, and any
lifecycle or `Close` requirement.

Write for a Go developer. Do not restate Webull's documentation; link to it for
authoritative broker semantics, and be explicit where Webull's own documentation
is ambiguous.

## Pull requests

- Branch from `main`. `main` is protected; direct pushes are rejected.
- CI must pass. It is a required status check.
- Merges are squashed, so the pull request title becomes the commit subject.
- Update [CHANGELOG.md](CHANGELOG.md) for anything user-visible.
- Update [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md) when adding coverage of
  a Webull API, and record decisions there rather than only in a pull request
  discussion — a decision that lives only in a review thread is lost.

If a change alters a decision recorded in the documentation, update every place
that decision appears. Restated facts drift.

## Reference material

Webull publishes official Python, Java and Go references, and they are useful
for understanding protocol behaviour. They are Apache-2.0 licensed and this
project is MIT, so:

- Protocol facts — endpoint paths, header names, signing steps, protobuf field
  numbers, enum wire values — must match. Being original about these means being
  wrong.
- Everything else — structure, naming, abstractions, documentation prose — is
  written fresh. Do not copy source or comments, and do not transliterate.

## Reporting security issues

See [SECURITY.md](SECURITY.md). Do not open a public issue for a vulnerability.
