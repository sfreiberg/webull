package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/proto"

	"github.com/sfreiberg/webull/internal/signing"
	"github.com/sfreiberg/webull/internal/streampb"
	"github.com/sfreiberg/webull/internal/transport"
)

// fakeMQTT is an in-memory paho client. It records subscribe HTTP calls
// through the real Doer but fakes the broker: Connect runs the OnConnect
// handler, and publish() feeds the default message handler.
type fakeMQTT struct {
	mqtt.Client
	opts       *mqtt.ClientOptions
	connectErr error

	mu        sync.Mutex
	connected bool
	onConnect mqtt.OnConnectHandler
	handler   mqtt.MessageHandler
}

func (f *fakeMQTT) Connect() mqtt.Token {
	f.mu.Lock()
	f.handler = f.opts.DefaultPublishHandler
	f.onConnect = f.opts.OnConnect
	if f.connectErr == nil {
		f.connected = true
	}
	oc := f.onConnect
	f.mu.Unlock()
	tok := &fakeToken{err: f.connectErr}
	if f.connectErr == nil && oc != nil {
		oc(f)
	}
	return tok
}

func (f *fakeMQTT) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.connected
}

func (f *fakeMQTT) Disconnect(uint) {
	f.mu.Lock()
	f.connected = false
	f.mu.Unlock()
}

// publish feeds a message to the client's handler as the broker would.
func (f *fakeMQTT) publish(topic string, payload []byte) {
	f.mu.Lock()
	h := f.handler
	f.mu.Unlock()
	if h != nil {
		h(f, &fakeMessage{topic: topic, payload: payload})
	}
}

// reconnect re-runs the OnConnect handler, as paho does after a drop.
func (f *fakeMQTT) reconnect() {
	f.mu.Lock()
	oc := f.onConnect
	f.mu.Unlock()
	if oc != nil {
		oc(f)
	}
}

type fakeToken struct{ err error }

func (t *fakeToken) Wait() bool                     { return true }
func (t *fakeToken) WaitTimeout(time.Duration) bool { return true }
func (t *fakeToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (t *fakeToken) Error() error { return t.err }

type fakeMessage struct {
	mqtt.Message
	topic   string
	payload []byte
}

func (m *fakeMessage) Topic() string     { return m.topic }
func (m *fakeMessage) Payload() []byte   { return m.payload }
func (m *fakeMessage) Qos() byte         { return 0 }
func (m *fakeMessage) Retained() bool    { return false }
func (m *fakeMessage) Duplicate() bool   { return false }
func (m *fakeMessage) MessageID() uint16 { return 0 }
func (m *fakeMessage) Ack()              {}

// fixture wires a Client to a fake broker and a real HTTP test server that
// records the subscribe/unsubscribe calls.
type fixture struct {
	client   *Client
	fake     *fakeMQTT
	subCalls []map[string]any
	subErr   int // HTTP status the subscribe endpoint returns; 0 means 200
	mu       sync.Mutex
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	fx := &fixture{}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(body, &m)
		m["_path"] = r.URL.Path
		fx.mu.Lock()
		fx.subCalls = append(fx.subCalls, m)
		status := fx.subErr
		fx.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error_code":"INVALID_SESSION","message":"nope"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)

	doer := &transport.Doer{
		HTTPClient: srv.Client(),
		Signer:     signing.New("k", "s"),
		DecodeError: func(r transport.Response) error {
			return &codedError{msg: string(r.Body)}
		},
		Retry: transport.RetryPolicy{MaxAttempts: 1},
	}
	host := strings.TrimPrefix(srv.URL, "https://")
	c := New(doer, host, "broker.invalid")
	c.newClient = func(o mqtt.ClientOptions) mqtt.Client {
		fx.fake = &fakeMQTT{opts: &o, connectErr: nil}
		return fx.fake
	}
	fx.client = c
	return fx
}

type codedError struct{ msg string }

func (e *codedError) Error() string { return e.msg }

func (fx *fixture) calls() []map[string]any {
	fx.mu.Lock()
	defer fx.mu.Unlock()
	return fx.subCalls
}

func snapshotPayload(t *testing.T) []byte {
	t.Helper()
	b, err := proto.Marshal(&streampb.Snapshot{
		Basic:    &streampb.Basic{Symbol: "AAPL", InstrumentId: "913256135", Timestamp: "1788300000000", TradingSession: "RTH"},
		Price:    "231.42",
		Volume:   "1000000",
		Change:   "1.10",
		OvnPrice: "231.00",
	})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestConnectAndReceiveSnapshot(t *testing.T) {
	fx := newFixture(t)
	stream, err := fx.client.Connect(context.Background(), ConnectOptions{SessionID: "sess-1"})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = stream.Close() }()
	if stream.SessionID() != "sess-1" {
		t.Errorf("session = %q", stream.SessionID())
	}

	if err := stream.Subscribe(context.Background(), SubscribeRequest{
		Symbols: []string{"AAPL"}, Types: []SubType{SubSnapshot}, Snapshot: true, Overnight: true,
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	call := fx.calls()[0]
	if call["_path"] != "/market-data/streaming/subscribe" || call["category"] != "US_STOCK" || call["grab"] != "true" || call["overnight_required"] != true {
		t.Errorf("subscribe call = %v", call)
	}

	fx.fake.publish("snapshot", snapshotPayload(t))
	msg, err := stream.Recv(context.Background())
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if msg.Type != TypeSnapshot || msg.Snapshot == nil {
		t.Fatalf("message = %+v", msg)
	}
	s := msg.Snapshot
	if s.Symbol != "AAPL" || !s.Price.Decimal.Equal(mustDec("231.42")) || s.Time.IsZero() || !s.OvernightPrice.Valid {
		t.Errorf("snapshot = %+v", s)
	}
}

func TestSubscribeValidation(t *testing.T) {
	fx := newFixture(t)
	stream, err := fx.client.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Close() }()
	if err := stream.Subscribe(context.Background(), SubscribeRequest{Symbols: []string{"AAPL"}}); err == nil {
		t.Error("missing Types must be rejected")
	}
	if err := stream.Unsubscribe(context.Background(), UnsubscribeRequest{Symbols: []string{"AAPL"}}); err == nil {
		t.Error("missing Types must be rejected unless All")
	}
}

func TestSubscribeErrorIsWrapped(t *testing.T) {
	fx := newFixture(t)
	fx.subErr = http.StatusExpectationFailed
	stream, err := fx.client.Connect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Close() }()
	err = stream.Subscribe(context.Background(), SubscribeRequest{Symbols: []string{"AAPL"}, Types: []SubType{SubSnapshot}})
	if !errors.Is(err, ErrSubscribeFailed) {
		t.Errorf("err = %v, want ErrSubscribeFailed", err)
	}
}

func TestConnectFailurePropagates(t *testing.T) {
	fx := newFixture(t)
	fx.client.newClient = func(o mqtt.ClientOptions) mqtt.Client {
		return &fakeMQTT{opts: &o, connectErr: errors.New("network down")}
	}
	if _, err := fx.client.Connect(context.Background()); err == nil {
		t.Error("a broker connect failure must propagate")
	}
}

func TestReconnectReplaysSubscriptions(t *testing.T) {
	fx := newFixture(t)
	stream, err := fx.client.Connect(context.Background(), ConnectOptions{SessionID: "s"})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stream.Close() }()
	if err := stream.Subscribe(context.Background(), SubscribeRequest{Symbols: []string{"AAPL", "MSFT"}, Types: []SubType{SubSnapshot, SubTick}}); err != nil {
		t.Fatal(err)
	}
	before := len(fx.calls())

	fx.fake.reconnect()

	// Wait for the resubscribe HTTP calls to land.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(fx.calls()) > before {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	replayed := fx.calls()[before:]
	if len(replayed) == 0 {
		t.Fatal("reconnect did not replay any subscription")
	}
	seen := map[string]bool{}
	for _, c := range replayed {
		if syms, ok := c["symbols"].([]any); ok {
			for _, s := range syms {
				seen[s.(string)] = true
			}
		}
	}
	if !seen["AAPL"] || !seen["MSFT"] {
		t.Errorf("replayed calls did not cover both symbols: %v", replayed)
	}
}

func TestUnsubscribeAllForgetsSubscriptions(t *testing.T) {
	fx := newFixture(t)
	stream, _ := fx.client.Connect(context.Background())
	defer func() { _ = stream.Close() }()
	_ = stream.Subscribe(context.Background(), SubscribeRequest{Symbols: []string{"AAPL"}, Types: []SubType{SubSnapshot}})
	if err := stream.Unsubscribe(context.Background(), UnsubscribeRequest{All: true}); err != nil {
		t.Fatal(err)
	}
	stream.mu.Lock()
	n := len(stream.subs)
	stream.mu.Unlock()
	if n != 0 {
		t.Errorf("unsubscribe all left %d subscriptions", n)
	}
}

func TestQueueDropsOldestWhenFull(t *testing.T) {
	fx := newFixture(t)
	stream, _ := fx.client.Connect(context.Background(), ConnectOptions{QueueSize: 2})
	defer func() { _ = stream.Close() }()
	// Three ticks into a queue of two: the first is dropped.
	for _, px := range []string{"1", "2", "3"} {
		b, _ := proto.Marshal(&streampb.Tick{Basic: &streampb.Basic{Symbol: "AAPL"}, Price: px})
		fx.fake.publish("tick", b)
	}
	got := []string{}
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		msg, err := stream.Recv(ctx)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, msg.Tick.Price.Decimal.String())
	}
	if got[0] != "2" || got[1] != "3" {
		t.Errorf("queue kept %v, want the newest two", got)
	}
}

func TestRecvUnblocksOnClose(t *testing.T) {
	fx := newFixture(t)
	stream, _ := fx.client.Connect(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := stream.Recv(context.Background())
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
	// A second Close is a no-op.
	if err := stream.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestDecodeEchoAndUnknownTopics(t *testing.T) {
	if m, err := decode("echo", []byte("x")); m != nil || err != nil {
		t.Errorf("echo must decode to nothing: %v %v", m, err)
	}
	m, err := decode("something-new", []byte("raw"))
	if err != nil || m.Type != "something-new" || string(m.Raw) != "raw" {
		t.Errorf("unknown topic = %+v %v", m, err)
	}
	if _, err := decode("snapshot", []byte("not-protobuf-\xff")); err == nil {
		t.Error("a malformed protobuf payload must error")
	}
}

func mustDec(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return d
}

func TestDecodeEveryMessageType(t *testing.T) {
	basic := &streampb.Basic{Symbol: "AAPL", InstrumentId: "913", Timestamp: "1788300000000", TradingSession: "RTH"}

	quote, _ := proto.Marshal(&streampb.Quote{
		Basic: basic,
		Asks:  []*streampb.AskBid{{Price: "231.5", Size: "10", Order: []*streampb.Order{{Mpid: "NSDQ", Size: "4"}}, Broker: []*streampb.Broker{{Bid: "B1", Name: "Broker One"}}}},
		Bids:  []*streampb.AskBid{{Price: "231.4", Size: "8"}},
	})
	tick, _ := proto.Marshal(&streampb.Tick{Basic: basic, Time: "1788300000500", Price: "231.45", Volume: "100", Side: "B"})
	esnap, _ := proto.Marshal(&streampb.EventSnapshot{Basic: basic, Price: "0.62", Volume: "40", LastTradeTime: "1788300000000", OpenInterest: "1000", YesAsk: "0.63", NoBid: "0.36"})
	equote, _ := proto.Marshal(&streampb.EventQuote{Basic: basic, YesBids: []*streampb.EventAskBid{{Price: "0.61", Size: "5"}}, NoBids: []*streampb.EventAskBid{{Price: "0.38", Size: "3"}}})
	etick, _ := proto.Marshal(&streampb.EventTick{Basic: basic, YesPrice: "0.62", NoPrice: "0.38", Volume: "5", Side: "YES", TradeId: "T1", Time: "1788300000500"})

	t.Run("quote", func(t *testing.T) {
		m, err := decode("quote", quote)
		if err != nil || m.Quote == nil || len(m.Quote.Asks) != 1 {
			t.Fatalf("quote = %+v %v", m, err)
		}
		a := m.Quote.Asks[0]
		if !a.Price.Decimal.Equal(mustDec("231.5")) || len(a.Orders) != 1 || a.Orders[0].MPID != "NSDQ" || len(a.Brokers) != 1 || a.Brokers[0].Name != "Broker One" {
			t.Errorf("ask level = %+v", a)
		}
		if len(m.Quote.Bids) != 1 || m.Quote.Symbol != "AAPL" {
			t.Errorf("quote = %+v", m.Quote)
		}
	})
	t.Run("tick", func(t *testing.T) {
		m, _ := decode("tick", tick)
		if m.Tick == nil || !m.Tick.Price.Decimal.Equal(mustDec("231.45")) || m.Tick.Side != "B" || m.Tick.Time.IsZero() {
			t.Errorf("tick = %+v", m.Tick)
		}
	})
	t.Run("event-snapshot", func(t *testing.T) {
		m, _ := decode("event-snapshot", esnap)
		if m.EventSnapshot == nil || !m.EventSnapshot.YesAsk.Decimal.Equal(mustDec("0.63")) || !m.EventSnapshot.OpenInterest.Valid || m.EventSnapshot.LastTradeTime.IsZero() {
			t.Errorf("event snapshot = %+v", m.EventSnapshot)
		}
	})
	t.Run("event-quote", func(t *testing.T) {
		m, _ := decode("event-quote", equote)
		if m.EventQuote == nil || len(m.EventQuote.YesBids) != 1 || len(m.EventQuote.NoBids) != 1 || !m.EventQuote.YesBids[0].Price.Decimal.Equal(mustDec("0.61")) {
			t.Errorf("event quote = %+v", m.EventQuote)
		}
	})
	t.Run("event-tick", func(t *testing.T) {
		m, _ := decode("event-tick", etick)
		if m.EventTick == nil || m.EventTick.TradeID != "T1" || !m.EventTick.YesPrice.Decimal.Equal(mustDec("0.62")) || m.EventTick.Time.IsZero() {
			t.Errorf("event tick = %+v", m.EventTick)
		}
	})
	t.Run("notice", func(t *testing.T) {
		m, _ := decode("notice", []byte("market halted"))
		if m.Type != TypeNotice || m.Notice != "market halted" {
			t.Errorf("notice = %+v", m)
		}
	})
	t.Run("malformed", func(t *testing.T) {
		for _, topic := range []string{"quote", "tick", "event-snapshot", "event-quote", "event-tick"} {
			if _, err := decode(topic, []byte("\xff\xff bad")); err == nil {
				t.Errorf("%s: a malformed payload must error", topic)
			}
		}
	})
}

func TestDecodeHelpers(t *testing.T) {
	if d := dec(""); d.Valid {
		t.Error("empty decimal must be invalid")
	}
	if d := dec("not-a-number"); d.Valid {
		t.Error("a bad decimal must be invalid, not an error")
	}
	if millis("").IsZero() != true || millis("0").IsZero() != true || millis("bad").IsZero() != true {
		t.Error("empty, zero and bad millis must all be zero")
	}
	if basic(nil) != (Basic{}) {
		t.Error("nil basic must be the zero value")
	}
}

func TestUnsubscribeSpecificForgets(t *testing.T) {
	fx := newFixture(t)
	stream, _ := fx.client.Connect(context.Background())
	defer func() { _ = stream.Close() }()
	_ = stream.Subscribe(context.Background(), SubscribeRequest{Symbols: []string{"AAPL", "MSFT"}, Types: []SubType{SubSnapshot}})
	if err := stream.Unsubscribe(context.Background(), UnsubscribeRequest{Symbols: []string{"AAPL"}, Types: []SubType{SubSnapshot}}); err != nil {
		t.Fatal(err)
	}
	stream.mu.Lock()
	n := len(stream.subs)
	stream.mu.Unlock()
	if n != 1 {
		t.Errorf("after unsubscribing AAPL, %d subscriptions remain, want 1 (MSFT)", n)
	}
}
