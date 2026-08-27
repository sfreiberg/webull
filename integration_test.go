package webull_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/sfreiberg/webull"
)

// Integration tests run against Webull's sandbox. They are not behind a build
// tag: they run automatically whenever credentials are present, and skip with
// a reported reason when they are not, so that a green test run never silently
// means "nothing was exercised".
//
// They are pinned to Sandbox and will refuse to run against Production. The
// failure mode being guarded against is placing real orders against a real
// account from a test run.

const integrationTimeout = 30 * time.Second

// newIntegrationClient returns a sandbox client, or skips the test.
func newIntegrationClient(t *testing.T) *webull.Client {
	t.Helper()

	key, secret := os.Getenv("WEBULL_APP_KEY"), os.Getenv("WEBULL_APP_SECRET")
	if key == "" || secret == "" {
		t.Skip("integration: WEBULL_APP_KEY and WEBULL_APP_SECRET are not set")
	}

	// A deliberate, redundant guard. Environment is hardcoded below, so this
	// can only fire if someone edits it, which is exactly when it is wanted.
	env := webull.Sandbox
	if env.IsProduction() {
		t.Fatal("integration tests must never run against production")
	}

	c, err := webull.NewClient(webull.Config{
		AppKey:      key,
		AppSecret:   secret,
		Environment: env,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func integrationContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), integrationTimeout)
	t.Cleanup(cancel)
	return ctx
}

// TestIntegrationSignatureIsAccepted is the test that matters most: it proves
// our reconstruction of Webull's signing algorithm produces signatures the
// server accepts. Every other request in the SDK depends on it.
func TestIntegrationSignatureIsAccepted(t *testing.T) {
	c := newIntegrationClient(t)

	enabled, err := c.TokenCheckEnabled(integrationContext(t))
	if err != nil {
		t.Fatalf("a signed request was rejected: %v", err)
	}
	t.Logf("token_check_enabled = %v", enabled)
}

func TestIntegrationEnvironmentIsSandbox(t *testing.T) {
	c := newIntegrationClient(t)
	if c.Environment().IsProduction() {
		t.Fatal("client is pointed at production")
	}
}

// TestIntegrationNotFoundIsClassified checks that a real gateway 404, which
// uses a different JSON shape from application errors, still classifies.
func TestIntegrationNotFoundIsClassified(t *testing.T) {
	c := newIntegrationClient(t)

	err := webull.ExportedGet(integrationContext(t), c, "/trading/definitely-not-a-real-endpoint", nil)
	if err == nil {
		t.Fatal("expected a 404")
	}
	if !errors.Is(err, webull.ErrNotFound) {
		t.Errorf("404 did not classify as ErrNotFound: %v", err)
	}

	var apiErr *webull.APIError
	if errors.As(err, &apiErr) {
		t.Logf("gateway error: status=%d code=%q message=%q", apiErr.StatusCode, apiErr.Code, apiErr.Message)
	}
}

func TestIntegrationUnauthenticatedIsClassified(t *testing.T) {
	if os.Getenv("WEBULL_APP_KEY") == "" {
		t.Skip("integration: WEBULL_APP_KEY is not set")
	}

	// Deliberately wrong credentials: the signature will not verify.
	c, err := webull.NewClient(webull.Config{
		AppKey:      "not-a-real-key",
		AppSecret:   "not-a-real-secret",
		Environment: webull.Sandbox,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := c.TokenCheckEnabled(integrationContext(t)); err == nil {
		t.Fatal("expected bad credentials to be rejected")
	} else {
		t.Logf("rejected as expected: %v", err)
	}
}
