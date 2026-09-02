package streaming_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/streaming"
)

// ExampleClient_Connect streams snapshots and ticks for one stock. The MQTT
// connection carries the data; Subscribe and Unsubscribe are signed HTTP
// calls that gate it, and a subscribe refused for a bad key is
// ErrSubscribeFailed.
func ExampleClient_Connect() {
	client, err := webull.NewClient(webull.Config{
		AppKey:      os.Getenv("WEBULL_APP_KEY"),
		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
		Environment: webull.Sandbox,
	})
	if err != nil {
		log.Fatal(err)
	}

	stream, err := client.Streaming.Connect(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = stream.Close() }()

	if err := stream.Subscribe(context.Background(), streaming.SubscribeRequest{
		Symbols: []string{"AAPL"},
		Types:   []streaming.SubType{streaming.SubSnapshot, streaming.SubTick},
	}); err != nil {
		if errors.Is(err, streaming.ErrSubscribeFailed) {
			log.Fatalf("subscribe refused: %v", err)
		}
		log.Fatal(err)
	}

	for {
		msg, err := stream.Recv(context.Background())
		if err != nil {
			break // the context was cancelled or the stream closed
		}
		switch {
		case msg.Type == streaming.TypeSnapshot && msg.Snapshot != nil:
			fmt.Println("snapshot", msg.Snapshot.Symbol, msg.Snapshot.Price)
		case msg.Type == streaming.TypeTick && msg.Tick != nil:
			fmt.Println("tick", msg.Tick.Symbol, msg.Tick.Price, msg.Tick.Volume)
		}
	}
}
