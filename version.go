// Package webull provides a Go client for the Webull OpenAPI.
//
// This is an independent open-source project and is not affiliated with,
// maintained by, or endorsed by Webull.
//
// The package is under active development and its public API is not yet
// stable.
package webull

import "runtime"

// Version is the semantic version of this SDK. It is reported in the
// User-Agent header of outgoing requests so that Webull can identify SDK
// traffic and so that bug reports can be tied to a specific release.
//
// Pre-release builds carry a -dev suffix.
const Version = "0.0.0-dev"

// userAgentPrefix identifies this SDK in the User-Agent header. It is kept
// separate from Version so that callers appending their own product token
// can be distinguished from the SDK's own identifier.
const userAgentPrefix = "webull-go"

// UserAgent returns the default User-Agent header value for SDK requests. It
// identifies the SDK, its version, and the Go runtime, following the
// conventional product/version format described in RFC 9110.
func UserAgent() string {
	return userAgentPrefix + "/" + Version + " (" + runtime.Version() + "; " + runtime.GOOS + "/" + runtime.GOARCH + ")"
}
