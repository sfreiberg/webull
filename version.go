// Package webull provides a Go client for the Webull OpenAPI.
//
// This is an independent open-source project and is not affiliated with,
// maintained by, or endorsed by Webull.
//
// Construct a Client with NewClient and use its service clients:
//
//	client, err := webull.NewClient(webull.Config{
//		AppKey:      os.Getenv("WEBULL_APP_KEY"),
//		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
//		Environment: webull.Sandbox,
//	})
//	accounts, err := client.Trade.Accounts(ctx)
//
// There is no default Environment: sandbox and production are different
// deployments with separately issued credentials, and a sandbox key returns
// "404 Route Not Found" for every production path.
//
// Every request is signed automatically. Failures from Webull are returned as
// *APIError, which matches the package's sentinel errors with errors.Is. The
// client never retries a POST, because in this API a replayed order is a
// duplicated order.
//
// The package is under active development and its public API is not yet
// stable.
package webull

import "runtime"

// Version is the semantic version of this SDK. It is reported in the
// User-Agent header of outgoing requests so that Webull can identify SDK
// traffic and so that bug reports can be tied to a specific release.
//
// Development builds between releases carry a -dev suffix.
const Version = "0.1.1-dev"

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
