package webull_test

// This file holds every Go code block from README.md, verbatim, so the
// compiler and linters keep the README honest: a snippet that drops an
// error, dereferences before checking, or references a renamed identifier
// fails the build. TestReadmeSnippetsMatchSource fails when a block here
// and its README twin drift apart, so editing either means editing both.
//
// The functions are referenced below but never called: the snippets reach
// the live API. Free identifiers a snippet assumes are its parameters, and
// the lines after a readme:end marker use up values the snippet
// deliberately leaves dangling.

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/sfreiberg/webull"
	"github.com/sfreiberg/webull/connect"
	"github.com/sfreiberg/webull/events"
	"github.com/sfreiberg/webull/marketdata"
	"github.com/sfreiberg/webull/streaming"
	"github.com/sfreiberg/webull/trade"
)

// Referenced so nothing reports the snippets unused; never invoked.
var _ = []any{
	readmeAuthentication,
	readmeQuickStart,
	readmePlaceOrder,
	readmeOptionOrder,
	readmeBracket,
	readmeVerticalSpread,
	readmeCryptoAndEvents,
	readmeReplace,
	readmeCancel,
	readmeMarketData,
	readmeTradeEvents,
	readmeStreaming,
	readmeConnect,
	readmeRateLimit,
}

func readmeAuthentication() {
	// readme:block
	client, err := webull.NewClient(webull.Config{
		AppKey:      os.Getenv("WEBULL_APP_KEY"),
		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
		Environment: webull.Sandbox, // or webull.Production
	})
	if err != nil {
		log.Fatal(err)
	}
	// readme:end
	_ = client
}

func readmeQuickStart(ctx context.Context) {
	// readme:block
	client, err := webull.NewClient(webull.Config{
		AppKey:      os.Getenv("WEBULL_APP_KEY"),
		AppSecret:   os.Getenv("WEBULL_APP_SECRET"),
		Environment: webull.Sandbox, // or webull.Production; there is no default
	})
	if err != nil {
		log.Fatal(err)
	}

	accounts, err := client.Trade.Accounts(ctx)
	if err != nil {
		log.Fatal(err)
	}
	acct := accounts[0]

	balance, err := client.Trade.Balance(ctx, acct.AccountID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(acct.AccountID, balance.TotalCashBalance)
	// readme:end
}

func readmePlaceOrder(ctx context.Context, client *webull.Client, acct trade.Account) {
	// readme:block
	order := &trade.Order{
		Symbol:     "AAPL",
		Side:       trade.Buy,
		Type:       trade.Limit,
		Quantity:   trade.Price("10"),
		LimitPrice: trade.Price("180.00"),
	}

	// Preview returns estimated cost and fees without placing anything.
	preview, err := client.Trade.PreviewOrder(ctx, acct.AccountID, order)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("estimated cost", preview.EstimatedCost)

	receipt, err := client.Trade.PlaceOrder(ctx, acct.AccountID, order)
	if err != nil {
		var apiErr *webull.APIError
		if errors.As(err, &apiErr) {
			log.Fatalf("rejected: %s (%s)", apiErr.Message, apiErr.Code)
		}
		// A transport failure means the outcome is unknown. The SDK generated
		// order.ClientOrderID before sending, so the order can be looked up
		// rather than blindly resent.
		log.Fatalf("outcome unknown for %s: %v", order.ClientOrderID, err)
	}
	fmt.Println("placed", receipt.OrderID)
	// readme:end
}

func readmeOptionOrder(ctx context.Context, client *webull.Client, acct trade.Account) {
	// readme:block
	chain, err := client.Trade.OptionContracts(ctx, trade.OptionContractsRequest{
		UnderlyingSymbols: []string{"AAPL"},
		OptionType:        trade.Call,
		StartDate:         "2026-12-18",
		EndDate:           "2026-12-18",
	})
	if err != nil {
		log.Fatal(err)
	}

	leg, err := trade.LegFromSymbol(chain.Contracts[0].Symbol) // e.g. AAPL261218C00240000
	if err != nil {
		log.Fatal(err)
	}

	receipt, err := client.Trade.PlaceOrder(ctx, acct.AccountID, &trade.Order{
		Symbol:         "AAPL",
		Side:           trade.Buy,
		Type:           trade.Limit,
		Quantity:       trade.Price("1"),
		LimitPrice:     trade.Price("5.50"),
		PositionIntent: trade.BuyToOpen,
		Legs:           []trade.OrderLeg{leg},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("placed", receipt.OrderID)
	// readme:end
}

func readmeBracket(ctx context.Context, client *webull.Client, acct trade.Account) {
	// readme:block
	combo := trade.Bracket(
		&trade.Order{Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit, TimeInForce: trade.GTC, Quantity: trade.Price("1"), LimitPrice: trade.Price("180.00")},
		&trade.Order{Symbol: "AAPL", Side: trade.Sell, Type: trade.Limit, TimeInForce: trade.GTC, Quantity: trade.Price("1"), LimitPrice: trade.Price("200.00")},   // take profit
		&trade.Order{Symbol: "AAPL", Side: trade.Sell, Type: trade.StopLoss, TimeInForce: trade.GTC, Quantity: trade.Price("1"), StopPrice: trade.Price("170.00")}, // stop loss
	)
	receipt, err := client.Trade.PlaceCombo(ctx, acct.AccountID, combo)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("placed group", receipt.ComboOrderID)

	// Later: CancelCombo cancels every order in the group still working; an
	// unfilled master takes its exits with it, while a filled master leaves
	// independent exit orders that this call still cancels.
	if err := client.Trade.CancelCombo(ctx, acct.AccountID, combo); err != nil {
		log.Fatal(err)
	}
	// readme:end
}

func readmeVerticalSpread(ctx context.Context, client *webull.Client, acct trade.Account) {
	// readme:block
	long, err := trade.LegFromSymbol("AAPL261218C00240000")
	if err != nil {
		log.Fatal(err)
	}
	short, err := trade.LegFromSymbol("AAPL261218C00250000")
	if err != nil {
		log.Fatal(err)
	}
	long.Side, short.Side = trade.Buy, trade.Sell

	receipt, err := client.Trade.PlaceOrder(ctx, acct.AccountID, &trade.Order{
		Symbol:         "AAPL",
		Side:           trade.Buy,
		Type:           trade.Limit,
		Quantity:       trade.Price("1"),    // spreads
		LimitPrice:     trade.Price("3.50"), // net debit
		OptionStrategy: trade.StrategyVertical,
		Legs:           []trade.OrderLeg{long, short},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("placed", receipt.OrderID)
	// readme:end
}

func readmeCryptoAndEvents(ctx context.Context, client *webull.Client, cryptoAcct, eventsAcct trade.Account) {
	// readme:block
	// Crypto, sized in coins (up to eight decimal places), $2 minimum.
	if _, err := client.Trade.PlaceOrder(ctx, cryptoAcct.AccountID, &trade.Order{
		InstrumentType: trade.InstrumentCrypto, Symbol: "BTCUSD",
		Side: trade.Buy, Type: trade.Limit, TimeInForce: trade.GTC,
		Quantity: trade.Price("0.001"), LimitPrice: trade.Price("60000"),
	}); err != nil {
		log.Fatal(err)
	}

	// Event contracts are limit-only and need the outcome being bought.
	if _, err := client.Trade.PlaceOrder(ctx, eventsAcct.AccountID, &trade.Order{
		InstrumentType: trade.InstrumentEvent, Symbol: "KXRATECUTCOUNT-26DEC31-T3",
		Side: trade.Buy, Type: trade.Limit,
		Quantity: trade.Price("5"), LimitPrice: trade.Price("0.10"),
		EventOutcome: trade.OutcomeYes,
	}); err != nil {
		log.Fatal(err)
	}
	// readme:end
}

func readmeReplace(ctx context.Context, client *webull.Client, acct trade.Account, order *trade.Order) {
	// readme:block
	if _, err := client.Trade.ReplaceOrder(ctx, acct.AccountID, trade.OrderModification{
		ClientOrderID: order.ClientOrderID,
		LimitPrice:    trade.Price("182.00"),
	}); err != nil {
		log.Fatal(err)
	}
	// readme:end
}

func readmeCancel(ctx context.Context, client *webull.Client, acct trade.Account, order *trade.Order) {
	// readme:block
	if _, err := client.Trade.CancelOrder(ctx, acct.AccountID, order.ClientOrderID); err != nil {
		log.Fatal(err)
	}
	// readme:end
}

func readmeMarketData(ctx context.Context, client *webull.Client) {
	// readme:block
	snaps, err := client.MarketData.Snapshots(ctx, marketdata.SnapshotsRequest{
		Symbols: []string{"AAPL", "SPY"}, ExtendedHours: true,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(snaps[0].Price.Decimal, snaps[0].LastTradeTime)

	bars, err := client.MarketData.Bars(ctx, marketdata.BarsRequest{
		Symbols: []string{"AAPL"}, Timespan: marketdata.Daily, Count: 30,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(len(bars[0].Bars), "daily bars")
	// readme:end
}

func readmeTradeEvents(ctx context.Context, client *webull.Client, acct trade.Account) {
	// readme:block
	stream, err := client.Events.Subscribe(ctx, events.SubscribeRequest{
		AccountIDs: []string{acct.AccountID},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = stream.Close() }()
	for {
		ev, err := stream.Recv()
		if err != nil {
			break
		}
		if ev.Kind == events.KindOrder && ev.Order != nil {
			fmt.Println(ev.Order.Scene, ev.Order.Symbol, ev.Order.FilledQuantity)
		}
	}
	// readme:end
}

func readmeStreaming(ctx context.Context, client *webull.Client) {
	// readme:block
	stream, err := client.Streaming.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = stream.Close() }()
	if err := stream.Subscribe(ctx, streaming.SubscribeRequest{
		Symbols: []string{"AAPL"},
		Types:   []streaming.SubType{streaming.SubSnapshot, streaming.SubTick},
	}); err != nil {
		log.Fatal(err)
	}
	for {
		msg, err := stream.Recv(ctx)
		if err != nil {
			break
		}
		if msg.Type == streaming.TypeTick && msg.Tick != nil {
			fmt.Println(msg.Tick.Symbol, msg.Tick.Price, msg.Tick.Volume)
		}
	}
	// readme:end
}

func readmeConnect(ctx context.Context, clientID, clientSecret, appKey, appSecret, state, code string) {
	// readme:block
	authorizer, err := connect.NewAuthorizer(connect.Config{
		ClientID: clientID, ClientSecret: clientSecret,
		AppKey: appKey, AppSecret: appSecret,
		Environment: webull.Production,
		RedirectURI: "https://app.example.com/webull/callback",
	})
	if err != nil {
		log.Fatal(err)
	}

	// Send the user here; on the callback, exchange the code.
	url := authorizer.AuthorizationURL(state)
	token, err := authorizer.ExchangeCode(ctx, code)
	if err != nil {
		log.Fatal(err)
	}

	// Trade on the user's behalf; the access token refreshes itself. Use
	// ClientFromStore to keep the rotating token pair in a store of your own.
	client, err := authorizer.Client(token)
	if err != nil {
		log.Fatal(err)
	}
	accounts, err := client.Trade.Accounts(ctx)
	if err != nil {
		log.Fatal(err)
	}
	// readme:end
	_ = url
	_ = accounts
}

func readmeRateLimit(err error) {
	// readme:block
	if errors.Is(err, webull.ErrRateLimited) {
		var apiErr *webull.APIError
		if errors.As(err, &apiErr) {
			wait := apiErr.RetryAfter
			if wait == 0 {
				wait = time.Second // Webull sent no Retry-After header
			}
			time.Sleep(wait)
		}
	}
	// readme:end
}
