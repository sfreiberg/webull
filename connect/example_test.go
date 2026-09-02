package connect_test

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/connect"
)

// Example shows the three-legged OAuth flow: send the user to authorize,
// exchange the returned code for a token, then trade on their behalf.
func Example() {
	authorizer, err := connect.NewAuthorizer(connect.Config{
		ClientID:     os.Getenv("WEBULL_CONNECT_CLIENT_ID"),
		ClientSecret: os.Getenv("WEBULL_CONNECT_CLIENT_SECRET"),
		AppKey:       os.Getenv("WEBULL_APP_KEY"),
		AppSecret:    os.Getenv("WEBULL_APP_SECRET"),
		Environment:  webull.Production,
		RedirectURI:  "https://app.example.com/webull/callback",
	})
	if err != nil {
		log.Fatal(err)
	}

	// 1. Send the user to the authorization URL. state is an unguessable
	// value stored in the session and checked against the redirect.
	url := authorizer.AuthorizationURL("random-state-per-session")
	fmt.Println("visit:", url)

	// 2. On the callback, exchange the one-time code for a token.
	code := "the-code-from-the-redirect-query"
	token, err := authorizer.ExchangeCode(context.Background(), code)
	if err != nil {
		log.Fatal(err)
	}

	// 3. Build a client and act on the user's account. The client refreshes
	// the access token before it expires; Token reads back the rotated pair
	// for persistence.
	client, err := authorizer.Client(token)
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

// ExampleAuthorizer_ClientFromStore shows keeping a user's token pair in a
// TokenStore, so their session survives the 30-minute access-token lifetime
// and process restarts. The store is scoped to one user and is the source of
// truth: it is read on first use and rotated pairs are saved back, so a pair
// already in the store is never overwritten with a stale one. A platform
// keeps one store per connected user.
func ExampleAuthorizer_ClientFromStore() {
	authorizer, err := connect.NewAuthorizer(connect.Config{
		ClientID:     os.Getenv("WEBULL_CONNECT_CLIENT_ID"),
		ClientSecret: os.Getenv("WEBULL_CONNECT_CLIENT_SECRET"),
		AppKey:       os.Getenv("WEBULL_APP_KEY"),
		AppSecret:    os.Getenv("WEBULL_APP_SECRET"),
		Environment:  webull.Production,
		RedirectURI:  "https://app.example.com/webull/callback",
	})
	if err != nil {
		log.Fatal(err)
	}

	// A real store persists to a database keyed by the user; this sketch
	// uses the in-memory one, seeded from a fresh authorization.
	token, err := authorizer.ExchangeCode(context.Background(), "the-code")
	if err != nil {
		log.Fatal(err)
	}
	store := connect.NewMemoryTokenStore(token)

	client, err := authorizer.ClientFromStore(store)
	if err != nil {
		log.Fatal(err)
	}
	accounts, err := client.Trade.Accounts(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(accounts))
}
