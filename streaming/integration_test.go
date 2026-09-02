package streaming_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sfreiberg/webull/internal/testutil"
	"github.com/sfreiberg/webull/streaming"
)

// TestIntegrationStreaming connects to the sandbox MQTT broker, subscribes
// to a snapshot, and waits for a message. A successful Connect and
// Subscribe verify the broker, TLS, session binding and the signed HTTP
// subscribe call live; whether data actually flows depends on the market
// being open, so the wait for a message skips rather than fails when the
// market is closed.
func TestIntegrationStreaming(t *testing.T) {
	client := testutil.NewIntegrationClient(t)
	ctx := testutil.IntegrationContext(t)

	stream, err := client.Streaming.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer stream.Close()
	t.Logf("connected: broker accepted session %s", stream.SessionID())

	if err := stream.Subscribe(ctx, streaming.SubscribeRequest{
		Symbols:  []string{"AAPL"},
		Types:    []streaming.SubType{streaming.SubSnapshot},
		Snapshot: true,
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	t.Log("subscribed: the signed subscribe call was accepted")

	type result struct {
		msg *streaming.Message
		err error
	}
	got := make(chan result, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			msg, err := stream.Recv(ctx)
			if err != nil {
				got <- result{nil, err}
				return
			}
			if msg.Type == streaming.TypeSnapshot {
				got <- result{msg, nil}
				return
			}
		}
	}()
	t.Cleanup(func() { _ = stream.Close(); <-done })

	select {
	case r := <-got:
		if r.err != nil {
			if errors.Is(r.err, context.DeadlineExceeded) || errors.Is(r.err, context.Canceled) {
				t.Skipf("integration: no snapshot before the deadline: %v", r.err)
			}
			t.Fatalf("Recv: %v", r.err)
		}
		if r.msg.Snapshot == nil || r.msg.Snapshot.Symbol == "" {
			t.Errorf("snapshot = %+v", r.msg.Snapshot)
		}
		t.Logf("received snapshot for %s", r.msg.Snapshot.Symbol)
	case <-time.After(15 * time.Second):
		t.Skip("integration: connected and subscribed, but no snapshot arrived in 15s (market likely closed)")
	}
}

// TestIntegrationUnsubscribeAll verifies the unsubscribe path against the
// live broker.
func TestIntegrationUnsubscribeAll(t *testing.T) {
	client := testutil.NewIntegrationClient(t)
	ctx := testutil.IntegrationContext(t)

	stream, err := client.Streaming.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer stream.Close()
	if err := stream.Subscribe(ctx, streaming.SubscribeRequest{Symbols: []string{"AAPL"}, Types: []streaming.SubType{streaming.SubSnapshot}}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := stream.Unsubscribe(ctx, streaming.UnsubscribeRequest{All: true}); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}
}
