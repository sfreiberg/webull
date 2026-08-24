# Changelog

All notable changes to this project are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Until 1.0.0 the public API may change in any release. Breaking changes are
called out explicitly.

## [Unreleased]

### Added

- API compatibility matrix and discovery documentation covering the Webull US
  OpenAPI surface, authentication, streaming protocols and wire format.
- Continuous integration: tests on Go 1.27 and 1.26, race detector, linting,
  vulnerability scanning, secret scanning, and an enforced 80% coverage floor.
- `Version` and `UserAgent` for identifying the SDK in outgoing requests.

### Notes

- No SDK functionality is implemented yet. The Broker API and FIX are out of
  scope; see [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md).

[Unreleased]: https://github.com/sfreiberg/webull/commits/main
