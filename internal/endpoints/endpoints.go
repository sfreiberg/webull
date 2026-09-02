// Package endpoints holds host constants that more than one package resolves,
// so that a host rename cannot drift between their tables.
package endpoints

// Connect API OAuth hosts, serving authorization, account and trading for the
// Connect API. The production host carries a "us-" prefix the sandbox host
// lacks, so the two are independent constants rather than a derived pattern.
const (
	ConnectProduction = "us-oauth-open-api.webull.com"
	ConnectSandbox    = "oauth-open-api.sandbox.webull.com"
)
