package webull_test

import (
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/internal/testutil"
)

// Integration tests run against Webull's sandbox. They are not behind a build
// tag: they run automatically whenever credentials are present, and skip with
// a reported reason when they are not, so that a green test run never silently
// means "nothing was exercised".
//
// They are pinned to Sandbox and will refuse to run against Production. The
// failure mode being guarded against is placing real orders against a real
// account from a test run.

// TestIntegrationSignatureIsAccepted is the test that matters most: it proves
// our reconstruction of Webull's signing algorithm produces signatures the
// server accepts. Every other request in the SDK depends on it.
func TestIntegrationSignatureIsAccepted(t *testing.T) {
	c := testutil.NewIntegrationClient(t)

	enabled, err := c.TokenCheckEnabled(testutil.IntegrationContext(t))
	if err != nil {
		t.Fatalf("a signed request was rejected: %v", err)
	}
	t.Logf("token_check_enabled = %v", enabled)
}

func TestIntegrationEnvironmentIsSandbox(t *testing.T) {
	c := testutil.NewIntegrationClient(t)
	if c.Environment().IsProduction() {
		t.Fatal("client is pointed at production")
	}
}

// TestIntegrationNotFoundIsClassified checks that a real gateway 404, which
// uses a different JSON shape from application errors, still classifies.
func TestIntegrationNotFoundIsClassified(t *testing.T) {
	c := testutil.NewIntegrationClient(t)

	err := webull.ExportedGet(testutil.IntegrationContext(t), c, "/trading/definitely-not-a-real-endpoint", nil)
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

	if _, err := c.TokenCheckEnabled(testutil.IntegrationContext(t)); err == nil {
		t.Fatal("expected bad credentials to be rejected")
	} else {
		t.Logf("rejected as expected: %v", err)
	}
}

// TestIntegrationAccessTokenLifecycle creates an access token and checks it
// back. Access tokens gate deployments where token authentication is enabled
// on top of signing; the sandbox reports it disabled, but the endpoints are
// documented to exist there, and the test environment skips the SMS
// verification step that production requires.
func TestIntegrationAccessTokenLifecycle(t *testing.T) {
	c := testutil.NewIntegrationClient(t)
	ctx := testutil.IntegrationContext(t)

	tok, err := c.CreateAccessToken(ctx)
	if err != nil {
		var apiErr *webull.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			t.Skipf("sandbox does not serve /auth/tokens/create: %v", err)
		}
		t.Fatalf("CreateAccessToken: %v", err)
	}
	if tok.Token == "" || tok.Status == "" {
		t.Fatalf("token = %+v", tok)
	}
	t.Logf("created token status=%s expires=%s", tok.Status, tok.ExpiresAt)

	checked, err := c.CheckAccessToken(ctx, tok.Token)
	if err != nil {
		t.Fatalf("CheckAccessToken: %v", err)
	}
	if checked.Status != tok.Status && checked.Status != webull.TokenNormal {
		t.Errorf("checked status = %s, created was %s", checked.Status, tok.Status)
	}
	t.Logf("checked token status=%s", checked.Status)
}
