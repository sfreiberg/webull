// Package testutil holds helpers shared by this module's tests.
package testutil

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/sfreiberg/webull"
)

// IntegrationTimeout bounds each integration test's context.
const IntegrationTimeout = 30 * time.Second

// NewIntegrationClient returns a sandbox client for integration tests, or
// skips the test with a reported reason when credentials are absent.
//
// It is pinned to Sandbox and refuses to run against Production. The failure
// mode being guarded against is placing real orders against a real account
// from a test run.
func NewIntegrationClient(t *testing.T) *webull.Client {
	t.Helper()

	key, secret := os.Getenv("WEBULL_APP_KEY"), os.Getenv("WEBULL_APP_SECRET")
	if key == "" || secret == "" {
		t.Skip("integration: WEBULL_APP_KEY and WEBULL_APP_SECRET are not set")
	}

	// Deliberate and redundant: Environment is hardcoded below, so this can
	// only fire if someone edits it, which is exactly when it is wanted.
	env := webull.Sandbox
	if env.IsProduction() {
		t.Fatal("integration tests must never run against production")
	}

	c, err := webull.NewClient(webull.Config{AppKey: key, AppSecret: secret, Environment: env})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// The default client pools keep-alive connections in the process-global
	// transport, and an idle HTTP/2 connection holds a read-loop goroutine.
	// Packages that verify goroutine leaks with goleak would report that
	// pool as a leak, so drain it when the test ends.
	t.Cleanup(func() {
		if tr, ok := http.DefaultTransport.(*http.Transport); ok {
			tr.CloseIdleConnections()
		}
	})
	if c.Environment().IsProduction() {
		t.Fatal("client is pointed at production")
	}
	return c
}

// IntegrationContext returns a context bounded by IntegrationTimeout and
// cancelled when the test ends.
func IntegrationContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), IntegrationTimeout)
	t.Cleanup(cancel)
	return ctx
}
