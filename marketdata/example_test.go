package marketdata_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/marketdata"
)

func exampleClient() *webull.Client {
	client, err := webull.NewClient(webull.Config{
		AppKey:      os.Getenv("WEBULL_APP_KEY"),
		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
		Environment: webull.Sandbox,
	})
	if err != nil {
		log.Fatal(err)
	}
	return client
}

// ExampleClient_Snapshots fetches the current state of two stocks, including
// the extended-hours session.
func ExampleClient_Snapshots() {
	client := exampleClient()

	snaps, err := client.MarketData.Snapshots(context.Background(), marketdata.SnapshotsRequest{
		Symbols:       []string{"AAPL", "SPY"},
		ExtendedHours: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range snaps {
		fmt.Println(s.Symbol, s.Price.Decimal, s.LastTradeTime)
	}
}

// ExampleClient_Bars fetches daily candles. Data the key is not subscribed to
// fails with ErrNotSubscribed, wrapping an *APIError that names the product.
func ExampleClient_Bars() {
	client := exampleClient()

	bars, err := client.MarketData.Bars(context.Background(), marketdata.BarsRequest{
		Symbols:  []string{"AAPL"},
		Timespan: marketdata.Daily,
		Count:    30,
	})
	if errors.Is(err, marketdata.ErrNotSubscribed) {
		log.Fatal("this key is not entitled to the requested data")
	}
	if err != nil {
		log.Fatal(err)
	}
	for _, b := range bars[0].Bars {
		fmt.Println(b.Time, b.Open, b.High, b.Low, b.Close, b.Volume)
	}
}
