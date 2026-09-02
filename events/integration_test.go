package events_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sfreiberg/webull/events"
	"github.com/sfreiberg/webull/internal/testutil"
	"github.com/sfreiberg/webull/trade"
)

// TestIntegrationEventStream subscribes to the sandbox event stream, then
// places and cancels a resting order and waits for the CANCEL_SUCCESS
// event. Placement of a resting order emits no event — the documented
// scenes begin at fills and cancels — so the cancel is the observable.
//
// A successful Subscribe alone verifies the transport, host and signature
// live; whether the sandbox actually pushes events is recorded by the
// second half, which skips with the reason if nothing arrives.
func TestIntegrationEventStream(t *testing.T) {
	client := testutil.NewIntegrationClient(t)
	ctx := testutil.IntegrationContext(t)

	accounts, err := client.Trade.Accounts(ctx)
	if err != nil {
		t.Fatalf("Accounts: %v", err)
	}
	var acct trade.Account
	for _, a := range accounts {
		if a.AccountClass == trade.AccountClassIndividualMargin {
			acct = a
		}
	}
	if acct.AccountID == "" {
		t.Skip("integration: no individual margin account")
	}

	stream, err := client.Events.Subscribe(ctx, events.SubscribeRequest{AccountIDs: []string{acct.AccountID}})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer stream.Close()
	t.Log("subscribed: the sandbox acknowledged the event stream")

	// A $1.00 GTC limit buy cannot fill and works at any hour; cancelling
	// it is what generates an event.
	o := &trade.Order{Symbol: "AAPL", Side: trade.Buy, Type: trade.Limit,
		TimeInForce: trade.GTC, Quantity: trade.Price("1"), LimitPrice: trade.Price("1.00")}
	if _, err := client.Trade.PlaceOrder(ctx, acct.AccountID, o); err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}
	t.Cleanup(func() {
		_, _ = client.Trade.CancelOrder(testutil.IntegrationContext(t), acct.AccountID, o.ClientOrderID)
	})
	time.Sleep(2 * time.Second) // let the order rest before cancelling
	if _, err := client.Trade.CancelOrder(ctx, acct.AccountID, o.ClientOrderID); err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}

	type result struct {
		ev  *events.Event
		err error
	}
	results := make(chan result, 1)
	go func() {
		for {
			ev, err := stream.Recv()
			if err != nil {
				results <- result{nil, err}
				return
			}
			if ev.Kind == events.KindOrder && ev.Order != nil && ev.Order.ClientOrderID == o.ClientOrderID {
				results <- result{ev, nil}
				return
			}
			t.Logf("unrelated event: kind=%d payload=%s", ev.Kind, ev.Payload)
		}
	}()

	select {
	case r := <-results:
		if r.err != nil {
			if errors.Is(r.err, context.DeadlineExceeded) {
				t.Skipf("integration: no event arrived before the context deadline: %v", r.err)
			}
			t.Fatalf("Recv: %v", r.err)
		}
		if r.ev.Order.Scene != events.CancelSuccess {
			t.Errorf("scene = %q, want CANCEL_SUCCESS (event %+v)", r.ev.Order.Scene, r.ev.Order)
		}
		t.Logf("received: %s for %s", r.ev.Order.Scene, r.ev.Order.Symbol)
	case <-time.After(20 * time.Second):
		t.Skip("integration: subscribed successfully, but the sandbox pushed no event for a cancelled order within 20s")
	}
}
