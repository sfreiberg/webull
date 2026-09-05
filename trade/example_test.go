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

// ExampleClient_PlaceCombo places a bracket: an entry order with a
// take-profit and a stop-loss that are activated once it fills, and are
// mutually cancelling. Bracket, OTO, OCO and OTOCO build a Combo from plain
// Orders; PlaceCombo submits it and CancelCombo cancels the whole group.
func ExampleClient_PlaceCombo() {
	client, accountID := exampleClient()

	combo := trade.Bracket(
		&trade.Order{Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit, TimeInForce: trade.GTC, Quantity: trade.Price("1"), LimitPrice: trade.Price("180.00")},
		&trade.Order{Symbol: "AAPL", Side: trade.Sell, Type: trade.Limit, TimeInForce: trade.GTC, Quantity: trade.Price("1"), LimitPrice: trade.Price("200.00")},   // take profit
		&trade.Order{Symbol: "AAPL", Side: trade.Sell, Type: trade.StopLoss, TimeInForce: trade.GTC, Quantity: trade.Price("1"), StopPrice: trade.Price("170.00")}, // stop loss
	)

	receipt, err := client.Trade.PlaceCombo(context.Background(), accountID, combo)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("placed group", receipt.ComboOrderID)

	// CancelCombo cancels every order in the group still working; an
	// unfilled master takes its exits with it, while a filled master
	// leaves independent exit orders that this call still cancels.
	if err := client.Trade.CancelCombo(context.Background(), accountID, combo); err != nil {
		log.Fatal(err)
	}
}

// ExampleClient_PlaceOrder_verticalSpread places a two-leg vertical spread.
// A multi-leg strategy is one Order carrying an OptionStrategy and its legs;
// the price is the net debit or credit for the spread.
func ExampleClient_PlaceOrder_verticalSpread() {
	client, accountID := exampleClient()

	long, err := trade.LegFromSymbol("AAPL261218C00240000")
	if err != nil {
		log.Fatal(err)
	}
	short, err := trade.LegFromSymbol("AAPL261218C00250000")
	if err != nil {
		log.Fatal(err)
	}
	long.Side, short.Side = trade.Buy, trade.Sell

	receipt, err := client.Trade.PlaceOrder(context.Background(), accountID, &trade.Order{
		Symbol:         "AAPL",
		Side:           trade.Buy,
		Type:           trade.Limit,
		TimeInForce:    trade.Day,
		Quantity:       trade.Price("1"),    // one spread
		LimitPrice:     trade.Price("3.50"), // net debit
		PositionIntent: trade.BuyToOpen,
		OptionStrategy: trade.StrategyVertical,
		Legs:           []trade.OrderLeg{long, short},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("placed", receipt.OrderID)
}
