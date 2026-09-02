package events_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/events"
)

// ExampleClient_Subscribe streams order events for one account. Subscribe
// blocks until the server acknowledges, so a returned stream is live; Recv
// handles heartbeats and reconnects internally, and returns the terminal
// errors ErrAuthFailed and ErrConnectionLimit rather than retrying them.
func ExampleClient_Subscribe() {
	client, err := webull.NewClient(webull.Config{
		AppKey:      os.Getenv("WEBULL_APP_KEY"),
		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
		Environment: webull.Sandbox,
	})
	if err != nil {
		log.Fatal(err)
	}
	accounts, err := client.Trade.Accounts(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	stream, err := client.Events.Subscribe(context.Background(), events.SubscribeRequest{
		AccountIDs: []string{accounts[0].AccountID},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = stream.Close() }()

	for {
		ev, err := stream.Recv()
		if err != nil {
			if errors.Is(err, events.ErrAuthFailed) || errors.Is(err, events.ErrConnectionLimit) {
				log.Fatalf("stream ended: %v", err)
			}
			break // the context was cancelled or the stream closed
		}
		if ev.Kind == events.KindOrder && ev.Order != nil {
			fmt.Println(ev.Order.Scene, ev.Order.Symbol, ev.Order.FilledQuantity)
		}
	}
}
