package connect_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/connect"
)

// TestIntegrationConnectFlow exercises the Connect OAuth flow against the
// live sandbox. Connect credentials are partner-gated and are not part of
// the standard sandbox key, so this skips unless a full set is present in
// the environment. The manual browser authorization step cannot be
// automated, so the authorization code is supplied out of band.
func TestIntegrationConnectFlow(t *testing.T) {
	clientID := os.Getenv("WEBULL_CONNECT_CLIENT_ID")
	clientSecret := os.Getenv("WEBULL_CONNECT_CLIENT_SECRET")
	appKey := os.Getenv("WEBULL_APP_KEY")
	appSecret := os.Getenv("WEBULL_APP_SECRET")
	redirect := os.Getenv("WEBULL_CONNECT_REDIRECT_URI")
	if clientID == "" || clientSecret == "" || appKey == "" || appSecret == "" || redirect == "" {
		t.Skip("integration: Connect credentials are not set (they are partner-gated)")
	}

	a, err := connect.NewAuthorizer(connect.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		AppKey:       appKey,
		AppSecret:    appSecret,
		Environment:  webull.Sandbox,
		RedirectURI:  redirect,
	})
	if err != nil {
		t.Fatalf("NewAuthorizer: %v", err)
	}

	// The authorization URL is always constructable; log it so a developer
	// can complete the browser step and feed the code back.
	t.Logf("authorize at: %s", a.AuthorizationURL("integration-state"))

	code := os.Getenv("WEBULL_CONNECT_AUTH_CODE")
	if code == "" {
		t.Skip("integration: set WEBULL_CONNECT_AUTH_CODE to a fresh code (60s lifetime) to exercise the token exchange")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tok, err := a.ExchangeCode(ctx, code)
	if err != nil {
		t.Fatalf("ExchangeCode: %v", err)
	}
	if tok.AccessToken == "" || tok.RefreshToken == "" {
		t.Fatalf("token = %+v", tok)
	}
	t.Logf("exchanged: access token expires %s, user %s", tok.AccessExpiry(), tok.IdentityID)

	client, err := a.Client(tok)
	if err != nil {
		t.Fatalf("Client: %v", err)
	}
	accounts, err := client.Trade.Accounts(ctx)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	t.Logf("reached %d account(s) as the authorized user", len(accounts))
}
