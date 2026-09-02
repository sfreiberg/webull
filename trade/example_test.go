package trade_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/trade"
)

// exampleClient is a small helper the examples share. In real code, build the
// client once and reuse it.
func exampleClient() (*webull.Client, string) {
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
	return client, accounts[0].AccountID
}

// ExampleClient_PlaceOrder places a limit order to buy ten shares. The SDK
// fills in what Webull requires but a caller should not have to know: the
// market, combo type, entrust type, trading session, and a client order ID.
func ExampleClient_PlaceOrder() {
	client, accountID := exampleClient()

	order := &trade.Order{
		Symbol:     "AAPL",
		Side:       trade.Buy,
		Type:       trade.Limit,
		Quantity:   trade.Price("10"),
		LimitPrice: trade.Price("180.00"),
	}

	receipt, err := client.Trade.PlaceOrder(context.Background(), accountID, order)
	if err != nil {
		var apiErr *webull.APIError
		if errors.As(err, &apiErr) {
			log.Fatalf("rejected: %s (%s)", apiErr.Message, apiErr.Code)
		}
		// A transport failure leaves the outcome unknown. The SDK wrote
		// order.ClientOrderID before sending, so the order can be looked up
		// rather than blindly resent.
		log.Fatalf("outcome unknown for %s: %v", order.ClientOrderID, err)
	}
	fmt.Println("placed", receipt.OrderID)
}

// ExampleClient_PreviewOrder estimates cost and fees without placing
// anything.
func ExampleClient_PreviewOrder() {
	client, accountID := exampleClient()

	preview, err := client.Trade.PreviewOrder(context.Background(), accountID, &trade.Order{
		Symbol:     "AAPL",
		Side:       trade.Buy,
		Type:       trade.Limit,
		Quantity:   trade.Price("10"),
		LimitPrice: trade.Price("180.00"),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(preview.EstimatedCost)
}

// ExampleClient_PlaceOrder_option buys a single option contract.
// LegFromSymbol parses the strike, expiry and right from an OCC symbol; the
// caller sets the side and quantity on the order.
func ExampleClient_PlaceOrder_option() {
	client, accountID := exampleClient()

	leg, err := trade.LegFromSymbol("AAPL261218C00240000")
	if err != nil {
		log.Fatal(err)
	}
	order := &trade.Order{
		Symbol:         "AAPL",
		Side:           trade.Buy,
		Type:           trade.Limit,
		TimeInForce:    trade.Day,
		Quantity:       trade.Price("1"),
		LimitPrice:     trade.Price("5.00"),
		PositionIntent: trade.BuyToOpen,
		Legs:           []trade.OrderLeg{leg},
	}
	receipt, err := client.Trade.PlaceOrder(context.Background(), accountID, order)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("placed", receipt.OrderID)
}

// ExampleClient_CancelOrder cancels a working order by its client order ID —
// the same value the SDK wrote onto the Order at placement.
func ExampleClient_CancelOrder() {
	client, accountID := exampleClient()

	order := &trade.Order{
		Symbol:      "AAPL",
		Side:        trade.Buy,
		Type:        trade.Limit,
		TimeInForce: trade.GTC,
		Quantity:    trade.Price("1"),
		LimitPrice:  trade.Price("1.00"), // far from market, so it rests
	}
	if _, err := client.Trade.PlaceOrder(context.Background(), accountID, order); err != nil {
		log.Fatal(err)
	}

	if _, err := client.Trade.CancelOrder(context.Background(), accountID, order.ClientOrderID); err != nil {
		log.Fatal(err)
	}
	fmt.Println("cancelled", order.ClientOrderID)
}

// ExampleLegFromSymbol parses an OCC option symbol into an order leg,
// setting the root symbol, expiry, right and strike. This example runs.
func ExampleLegFromSymbol() {
	leg, err := trade.LegFromSymbol("AAPL261218C00240000")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s %s %s $%s\n", leg.Symbol, leg.OptionType, leg.ExpireDate, leg.StrikePrice.Decimal)
	// Output: AAPL CALL 2026-12-18 $240
}

// ExamplePrice builds a decimal quantity or price from a string, avoiding
// the precision loss of a float literal.
func ExamplePrice() {
	q := trade.Price("10.5")
	fmt.Println(q.Decimal)
	// Output: 10.5
}
