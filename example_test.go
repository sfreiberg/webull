package webull_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/sfreiberg/webull"
)

// Example shows the entry point: construct a client, then reach a service
// through it. Credentials come from the Webull developer portal; sandbox
// and production keys are separate.
func Example() {
	client, err := webull.NewClient(webull.Config{
		AppKey:      os.Getenv("WEBULL_APP_KEY"),
		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
		Environment: webull.Sandbox, // or webull.Production; there is no default
	})
	if err != nil {
		log.Fatal(err)
	}

	accounts, err := client.Trade.Accounts(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	for _, a := range accounts {
		fmt.Println(a.AccountID, a.AccountClass)
	}
}

// ExampleNewClient supplies a custom User-Agent identifying the calling
// application, which is appended to the SDK's own.
func ExampleNewClient() {
	client, err := webull.NewClient(webull.Config{
		AppKey:      os.Getenv("WEBULL_APP_KEY"),
		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
		Environment: webull.Sandbox,
		UserAgent:   "acme-trader/1.4",
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = client
}

// ExampleAPIError shows matching a failure against the sentinel errors.
// Every failure from Webull is an *APIError carrying the HTTP status,
// Webull's code and message, and a request ID for support.
func ExampleAPIError() {
	client, err := webull.NewClient(webull.Config{
		AppKey:      os.Getenv("WEBULL_APP_KEY"),
		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
		Environment: webull.Sandbox,
	})
	if err != nil {
		log.Fatal(err)
	}

	_, err = client.Trade.Accounts(context.Background())
	if errors.Is(err, webull.ErrRateLimited) {
		var apiErr *webull.APIError
		if errors.As(err, &apiErr) {
			fmt.Printf("rate limited; retry after %s\n", apiErr.RetryAfter)
		}
	}
}

// ExampleClient_CreateAccessToken shows the token flow for deployments where
// TokenCheckEnabled reports that token authentication is required.
func ExampleClient_CreateAccessToken() {
	client, err := webull.NewClient(webull.Config{
		AppKey:      os.Getenv("WEBULL_APP_KEY"),
		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
		Environment: webull.Sandbox,
	})
	if err != nil {
		log.Fatal(err)
	}

	needed, err := client.TokenCheckEnabled(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	if !needed {
		return // signing alone is enough for this deployment
	}

	// The token starts PENDING until verified through SMS and the Webull
	// app; it lives 15 days and is then recreated, not refreshed.
	tok, err := client.CreateAccessToken(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(tok.Status, tok.ExpiresAt)

	// Requests carry the token once a client is built with it.
	authed, err := webull.NewClient(webull.Config{
		AppKey:      os.Getenv("WEBULL_APP_KEY"),
		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
		Environment: webull.Sandbox,
		AccessToken: tok.Token,
	})
	if err != nil {
		log.Fatal(err)
	}
	_ = authed
}
