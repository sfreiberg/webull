# webull

[![CI](https://github.com/sfreiberg/webull/actions/workflows/ci.yml/badge.svg)](https://github.com/sfreiberg/webull/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/sfreiberg/webull/graph/badge.svg)](https://codecov.io/gh/sfreiberg/webull)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

An independent, open-source Go SDK for the [Webull OpenAPI](https://developer.webull.com/).

> **Status: under construction.** This project is in early development. There is
> no usable release yet, and the public API is expected to change without notice
> until v1.0.0.

## Disclaimer

This is an independent open-source project. It is not affiliated with,
maintained by, authorized by, or endorsed by Webull. "Webull" and related marks
belong to their respective owners.

Trading involves risk. This software is provided without warranty of any kind.
You are responsible for any orders it places on your behalf.

## Documentation

- [API compatibility matrix](docs/COMPATIBILITY.md) — what is implemented, what is planned, and what is out of scope
- [Discovery findings](docs/discovery/) — inventory of the Webull OpenAPI surface, authentication, streaming and wire format

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md) for build,
test and code standards, and [SECURITY.md](SECURITY.md) for reporting
vulnerabilities.

## Installation

```
go get github.com/sfreiberg/webull
```

## License

[MIT](LICENSE)
