# Security policy

## Reporting a vulnerability

**Please do not open a public issue for a security vulnerability.**

Report it privately through GitHub:

1. Go to the [Security tab](https://github.com/sfreiberg/webull/security).
2. Choose **Report a vulnerability**.

This opens a private advisory visible only to the maintainers. It needs no email
address and no prior contact.

Please include what the issue is, how to reproduce it, which versions are
affected, and what an attacker could do with it. A proof of concept helps.

You will get an acknowledgement within a week. If a fix is warranted, the
advisory will track it, and you will be credited in the release notes unless you
would rather not be.

## Scope

This project is a client library. It holds credentials, signs requests, and
places trades on a user's behalf, so the security-relevant surface is roughly:

**In scope**

- Leaking credentials, tokens or signing material into logs, error messages or
  panics.
- Flaws in request signing that would let a request be forged or replayed.
- Failures in TLS verification, or any path that transmits credentials without
  encryption.
- Vulnerabilities that could cause an order to be placed, modified or cancelled
  other than as the caller instructed.
- Dependency vulnerabilities reachable through this library's code paths.

**Out of scope**

- Vulnerabilities in Webull's own services. Report those to Webull.
- Anything requiring an attacker who already controls the machine running your
  code, or who already holds your API credentials.
- Missing hardening that has no demonstrable impact.

## Supported versions

The project is pre-1.0. Fixes land on `main` and in the next release; there are
no maintained release branches yet. This section will be replaced with a version
table at 1.0.0.

## Handling credentials

Some notes for anyone using this library, since most real incidents come from
credential handling rather than library flaws:

- Keep your app key and app secret out of source control. This repository scans
  every pull request with `gitleaks`, but your repository may not.
- The SDK never writes credentials to disk. If you persist tokens, that storage
  is yours to secure.
- Debug logging redacts credentials and tokens. If you find output that does
  not, that is a vulnerability under this policy — please report it.
- Prefer environment variables or a secret manager over configuration files.
