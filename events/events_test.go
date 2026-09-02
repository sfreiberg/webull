package events

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/sfreiberg/webull/internal/eventspb"
	"github.com/sfreiberg/webull/internal/signing"
)

// fakeServer scripts EventService responses per connection attempt.
type fakeServer struct {
	eventspb.UnimplementedEventServiceServer

	mu      sync.Mutex
	calls   int
	gotMD   []metadata.MD
	gotReqs []*eventspb.SubscribeRequest

	// handler drives one Subscribe call; call counts from 1.
	handler func(call int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error
}

func (f *fakeServer) Subscribe(req *eventspb.SubscribeRequest, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
	md, _ := metadata.FromIncomingContext(stream.Context())
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.gotMD = append(f.gotMD, md)
	f.gotReqs = append(f.gotReqs, req)
	f.mu.Unlock()
	return f.handler(call, stream)
}

func (f *fakeServer) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// newFixture starts an in-process EventService and returns a Client wired
// to it over bufconn.
func newFixture(t *testing.T, f *fakeServer) *Client {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	eventspb.RegisterEventServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	c := New(signing.New("k", "s"), "ignored")
	c.host = "passthrough:///bufnet"
	c.dialOpts = []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
	}
	return c
}

func ack() *eventspb.SubscribeResponse {
	return &eventspb.SubscribeResponse{EventType: eventspb.EventType_SubscribeSuccess}
}

func ping() *eventspb.SubscribeResponse {
	return &eventspb.SubscribeResponse{EventType: eventspb.EventType_Ping, ContentType: "text/plain"}
}

func orderEvent(payload string) *eventspb.SubscribeResponse {
	return &eventspb.SubscribeResponse{
		EventType:     eventspb.EventType(KindOrder),
		SubscribeType: uint32(Orders),
		ContentType:   "application/json",
		Payload:       payload,
		RequestId:     "req-1",
		Timestamp:     1788300000000,
	}
}

const orderPayload = `{"request_id":"req-1","account_id":"ACCT","client_order_id":"co-1","order_id":"o-1",` +
	`"instrument_id":"913256135","order_status":"FILLED","symbol":"AAPL","qty":"10.00","filled_price":"180.00",` +
	`"filled_qty":"10.00","filled_time":"2026-09-02T06:27:43.312+0000","side":"BUY","scene_type":"FINAL_FILLED",` +
	`"category":"US_STOCK","order_type":"LIMIT"}`

func TestSubscribeSignsAndDeliversTypedOrderEvents(t *testing.T) {
	f := &fakeServer{handler: func(_ int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ack())
		_ = stream.Send(ping())
		_ = stream.Send(orderEvent(orderPayload))
		return nil
	}}
	c := newFixture(t, f)

	stream, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"ACCT"}, Types: []SubscriptionType{Orders, Positions}})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer func() { _ = stream.Close() }()

	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if ev.Kind != KindOrder || ev.RequestID != "req-1" || ev.Time.IsZero() {
		t.Errorf("event = %+v", ev)
	}
	o := ev.Order
	if o == nil {
		t.Fatalf("order not decoded; payload = %s", ev.Payload)
	}
	if o.Scene != FinalFilled || o.Symbol != "AAPL" || !o.FilledPrice.Decimal.Equal(decimalFrom(t, "180.00")) || o.FilledTime.IsZero() {
		t.Errorf("order = %+v", o)
	}

	// The subscription request carried the account, the type mask and the
	// signature metadata.
	req := f.gotReqs[0]
	if req.GetSubscribeType() != uint32(Orders|Positions) || len(req.GetAccounts()) != 1 || req.GetAccounts()[0] != "ACCT" {
		t.Errorf("request = %+v", req)
	}
	md := f.gotMD[0]
	for _, k := range []string{"x-app-key", "x-signature", "x-signature-nonce", "x-signature-algorithm", "x-signature-version", "x-timestamp"} {
		if len(md.Get(k)) != 1 || md.Get(k)[0] == "" {
			t.Errorf("metadata %s missing: %v", k, md)
		}
	}
}

func TestSubscribeDefaultsToAllTypes(t *testing.T) {
	f := &fakeServer{handler: func(_ int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ack())
		return nil
	}}
	c := newFixture(t, f)
	stream, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"ACCT"}})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer func() { _ = stream.Close() }()
	if got := f.gotReqs[0].GetSubscribeType(); got != 7 {
		t.Errorf("default mask = %d, want 7", got)
	}
}

func TestSubscribeRequiresAccounts(t *testing.T) {
	c := newFixture(t, &fakeServer{})
	if _, err := c.Subscribe(context.Background(), SubscribeRequest{}); err == nil {
		t.Error("empty AccountIDs must be rejected")
	}
}

func TestAuthErrorIsTerminal(t *testing.T) {
	f := &fakeServer{handler: func(_ int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(&eventspb.SubscribeResponse{EventType: eventspb.EventType_AuthError, Payload: "bad key"})
		return nil
	}}
	c := newFixture(t, f)
	_, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}})
	if !errors.Is(err, ErrAuthFailed) {
		t.Errorf("err = %v, want ErrAuthFailed", err)
	}
}

func TestConnectionLimitMidStreamIsTerminal(t *testing.T) {
	f := &fakeServer{handler: func(_ int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ack())
		_ = stream.Send(&eventspb.SubscribeResponse{EventType: eventspb.EventType_NumOfConnExceed})
		return nil
	}}
	c := newFixture(t, f)
	stream, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer func() { _ = stream.Close() }()
	if _, err := stream.Recv(); !errors.Is(err, ErrConnectionLimit) {
		t.Errorf("err = %v, want ErrConnectionLimit", err)
	}
}

func TestExpiredSubscriptionResubscribes(t *testing.T) {
	f := &fakeServer{handler: func(call int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ack())
		if call == 1 {
			_ = stream.Send(&eventspb.SubscribeResponse{EventType: eventspb.EventType_SubscribeExpired})
			return nil
		}
		_ = stream.Send(orderEvent(orderPayload))
		return nil
	}}
	c := newFixture(t, f)
	stream, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}, ReconnectDelay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer func() { _ = stream.Close() }()
	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv after expiry: %v", err)
	}
	if ev.Kind != KindOrder || f.callCount() != 2 {
		t.Errorf("event = %+v, calls = %d", ev, f.callCount())
	}
}

func TestTransientFailureReconnects(t *testing.T) {
	f := &fakeServer{handler: func(call int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ack())
		if call == 1 {
			return status.Error(codes.Unavailable, "restarting")
		}
		_ = stream.Send(orderEvent(orderPayload))
		return nil
	}}
	c := newFixture(t, f)
	stream, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}, ReconnectDelay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer func() { _ = stream.Close() }()
	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv after reconnect: %v", err)
	}
	if ev.Kind != KindOrder || f.callCount() != 2 {
		t.Errorf("event = %+v, calls = %d", ev, f.callCount())
	}
}

func TestNonRetryableStatusEndsTheStream(t *testing.T) {
	f := &fakeServer{handler: func(_ int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ack())
		return status.Error(codes.PermissionDenied, "no")
	}}
	c := newFixture(t, f)
	stream, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}, ReconnectDelay: 10 * time.Millisecond})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer func() { _ = stream.Close() }()
	if _, err := stream.Recv(); err == nil || f.callCount() != 1 {
		t.Errorf("err = %v, calls = %d; a non-retryable status must not reconnect", err, f.callCount())
	}
}

func TestReconnectAttemptsAreBounded(t *testing.T) {
	f := &fakeServer{handler: func(call int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ack())
		return status.Error(codes.Unavailable, "still down")
	}}
	c := newFixture(t, f)
	stream, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}, ReconnectDelay: time.Millisecond, MaxReconnectAttempts: 2})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer func() { _ = stream.Close() }()
	_, err = stream.Recv()
	if err == nil || !strings.Contains(err.Error(), "gave up") {
		t.Errorf("err = %v, want reconnect exhaustion", err)
	}
	// Initial connect + two reconnects.
	if f.callCount() != 3 {
		t.Errorf("calls = %d, want 3", f.callCount())
	}
}

func TestCloseEndsRecv(t *testing.T) {
	release := make(chan struct{})
	f := &fakeServer{handler: func(_ int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ack())
		<-release
		return nil
	}}
	defer close(release)
	c := newFixture(t, f)
	stream, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := stream.Recv()
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	_ = stream.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Error("Recv must fail after Close")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Recv did not return after Close")
	}
}

func TestDecodeEventShapes(t *testing.T) {
	// Non-JSON content is carried raw.
	ev := decodeEvent(KindOrder, "text/plain", []byte("raw"), "r", 0)
	if ev.Order != nil || string(ev.Payload) != "raw" || !ev.Time.IsZero() {
		t.Errorf("plain event = %+v", ev)
	}
	// Malformed JSON keeps the payload without a typed view.
	ev = decodeEvent(KindOrder, "application/json", []byte("{"), "r", 0)
	if ev.Order != nil || string(ev.Payload) != "{" {
		t.Errorf("malformed event = %+v", ev)
	}
	// A position event decodes the documented settlement shape.
	ev = decodeEvent(KindPosition, "application/json",
		[]byte(`{"event_name":"Rate cuts","settle_side":"Yes","quantity":"40","settle_amount":"40.00"}`), "r", 1788300000000)
	if ev.Position == nil || ev.Position.EventName != "Rate cuts" || !ev.Position.Quantity.Decimal.Equal(decimalFrom(t, "40")) {
		t.Errorf("position event = %+v", ev.Position)
	}
	// An option event is raw-only: Webull documents no payload shape.
	ev = decodeEvent(KindOption, "application/json", []byte(`{"x":1}`), "r", 0)
	if ev.Order != nil || ev.Position != nil || len(ev.Payload) == 0 {
		t.Errorf("option event = %+v", ev)
	}
}

func decimalFrom(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestUnexpectedFirstResponseFailsSubscribe(t *testing.T) {
	f := &fakeServer{handler: func(_ int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ping()) // no acknowledgement first
		return nil
	}}
	c := newFixture(t, f)
	if _, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}}); err == nil {
		t.Error("a stream that does not acknowledge must fail Subscribe")
	}
}

func TestSubscribeAgainstDeadServerFails(t *testing.T) {
	f := &fakeServer{handler: func(_ int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error { return nil }}
	c := newFixture(t, f)
	// Replace the dialer with one that cannot connect.
	c.dialOpts = []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return nil, errors.New("no route")
		}),
	}
	if _, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}}); err == nil {
		t.Error("an unreachable server must fail Subscribe")
	}
}

func TestReconnectRetriesThroughAnOutage(t *testing.T) {
	f := &fakeServer{handler: func(_ int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ack())
		return status.Error(codes.Unavailable, "going away")
	}}
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	eventspb.RegisterEventServiceServer(srv, f)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	// The dialer serves exactly one connection, so every reconnect
	// attempt fails; the stream must keep retrying until the budget is
	// spent rather than dying at the first failed dial.
	var dials atomic.Int32
	c := New(signing.New("k", "s"), "ignored")
	c.host = "passthrough:///bufnet"
	c.dialOpts = []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			if dials.Add(1) > 1 {
				return nil, errors.New("network is down")
			}
			return lis.DialContext(ctx)
		}),
	}

	stream, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}, ReconnectDelay: 5 * time.Millisecond, MaxReconnectAttempts: 3})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer func() { _ = stream.Close() }()
	_, err = stream.Recv()
	if err == nil || !strings.Contains(err.Error(), "gave up") {
		t.Errorf("a persistent outage must exhaust the attempt budget, got %v", err)
	}
	// The initial connect plus retries against the dead dialer: only the
	// first connection ever succeeded.
	if dials.Load() < 2 {
		t.Errorf("dials = %d; the outage must have been retried", dials.Load())
	}
}

func TestCloseDuringReconnectDelay(t *testing.T) {
	f := &fakeServer{handler: func(_ int, stream grpc.ServerStreamingServer[eventspb.SubscribeResponse]) error {
		_ = stream.Send(ack())
		return status.Error(codes.Unavailable, "down")
	}}
	c := newFixture(t, f)
	stream, err := c.Subscribe(context.Background(), SubscribeRequest{AccountIDs: []string{"A"}, ReconnectDelay: time.Hour})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := stream.Recv()
		done <- err
	}()
	time.Sleep(30 * time.Millisecond)
	if err := stream.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Recv did not return during the reconnect delay")
	}
	if err := stream.Close(); err != nil {
		t.Errorf("second Close must be a no-op, got %v", err)
	}
}

func TestRequestDefaults(t *testing.T) {
	if d := (SubscribeRequest{}).delay(); d != 5*time.Second {
		t.Errorf("default delay = %v", d)
	}
	if d := (SubscribeRequest{ReconnectDelay: time.Second}).delay(); d != time.Second {
		t.Errorf("custom delay = %v", d)
	}
	if m := (SubscribeRequest{Types: []SubscriptionType{Options}}).mask(); m != 4 {
		t.Errorf("mask = %d", m)
	}
}
