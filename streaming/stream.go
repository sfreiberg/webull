package streaming

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Stream is a live market-data connection. Recv is not safe for concurrent
// use; Subscribe, Unsubscribe and Close are.
type Stream struct {
	client  *Client
	session string
	opts    ConnectOptions

	conn  mqtt.Client
	queue chan *Message

	// subMu serializes every subscription operation — a user Subscribe or
	// Unsubscribe and the automatic resubscribe after a reconnect — across
	// both its HTTP call and its update of subs, so a resubscribe cannot
	// interleave with a concurrent Unsubscribe and revive a forgotten key.
	subMu sync.Mutex
	subs  map[subKey]subscription

	// dropped counts messages discarded because the queue was full or a
	// payload failed to decode, so a consumer can detect a gap.
	dropped atomic.Uint64

	closeOnce sync.Once
	closed    chan struct{}
}

// subKey identifies a subscription for replay and removal.
type subKey struct {
	symbol   string
	category Category
	subType  SubType
}

type subscription struct {
	depth     int
	snapshot  bool
	overnight bool
}

// connect opens the broker connection and blocks until it is established or
// the context ends. Auto-reconnect keeps a once-established connection up,
// but the initial connect does not retry, so a failure is returned rather
// than blocking indefinitely.
func (s *Stream) connect(ctx context.Context) error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("ssl://%s:%d", s.client.brokerHost, s.client.brokerPort))
	opts.SetClientID(s.session)
	opts.SetUsername(s.client.doer.Signer.AppKey)
	opts.SetPassword(newSessionID()) // the broker ignores the password; the app key authenticates
	opts.SetTLSConfig(tlsConfig())
	opts.SetCleanSession(true)
	opts.SetConnectTimeout(s.opts.ConnectTimeout)
	// Reconnect a once-established connection, but do not retry the initial
	// connect: with connect-retry on, paho loops forever on a down or
	// refused broker and the Connect token never completes.
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(s.opts.ReconnectDelay)

	opts.SetDefaultPublishHandler(func(_ mqtt.Client, m mqtt.Message) {
		msg, err := decode(m.Topic(), m.Payload())
		if err != nil || msg == nil {
			// A malformed payload or the echo topic: count a decode
			// failure but keep the stream alive rather than surfacing it
			// as a terminal error.
			if err != nil {
				s.dropped.Add(1)
			}
			return
		}
		s.enqueue(msg)
	})
	// On every (re)connection, replay the subscription set so a dropped
	// connection resumes streaming what it had. The handler runs on paho's
	// goroutine with no caller context, so resubscribe makes its own.
	opts.SetOnConnectHandler(func(mqtt.Client) { //nolint:contextcheck // callback has no context
		s.resubscribe()
	})

	s.conn = s.client.newClient(*opts)

	token := s.conn.Connect()
	if !waitToken(ctx, token) {
		s.conn.Disconnect(0)
		return ctx.Err()
	}
	// token.Error() carries paho's connect failures: a network error or a
	// standard MQTT CONNACK refusal. Webull's non-standard CONNACK codes
	// are not surfaced by paho (see ErrSubscribeFailed), so a well-formed
	// key reaches this point connected.
	if err := token.Error(); err != nil {
		s.conn.Disconnect(0)
		return fmt.Errorf("streaming: connect: %w", err)
	}
	return nil
}

// waitToken blocks until the token completes or the context is done,
// reporting whether the token completed.
func waitToken(ctx context.Context, token mqtt.Token) bool {
	select {
	case <-token.Done():
		return true
	case <-ctx.Done():
		return false
	}
}

// Recv returns the next message. It blocks until one arrives, the context
// is cancelled, or the stream is closed. A payload that fails to decode is
// counted (see Dropped) rather than returned, so a single bad message does
// not end a healthy stream.
func (s *Stream) Recv(ctx context.Context) (*Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-s.closed:
		return nil, errors.New("streaming: stream closed")
	case msg := <-s.queue:
		return msg, nil
	}
}

// Dropped returns the number of messages discarded because the queue was
// full or a payload failed to decode. A rising count means the consumer is
// not keeping up, or the wire schema has drifted.
func (s *Stream) Dropped() uint64 { return s.dropped.Load() }

// enqueue adds a message, dropping the oldest if the queue is full so a
// slow consumer cannot block paho's message goroutine. Messages arrive on
// that single goroutine (order preserved), so one send plus at most one
// eviction suffices.
func (s *Stream) enqueue(msg *Message) {
	select {
	case s.queue <- msg:
		return
	default:
	}
	select {
	case <-s.queue:
		s.dropped.Add(1)
	default:
	}
	select {
	case s.queue <- msg:
	default:
	}
}

// remember records subscriptions for replay on reconnect.
func (s *Stream) remember(req SubscribeRequest) {
	for _, sym := range req.Symbols {
		for _, st := range req.Types {
			s.subs[subKey{sym, category(req.Category), st}] = subscription{
				depth: req.Depth, snapshot: req.Snapshot, overnight: req.Overnight,
			}
		}
	}
}

// forget drops subscriptions cancelled by an Unsubscribe.
func (s *Stream) forget(req UnsubscribeRequest) {
	if req.All {
		s.subs = map[subKey]subscription{}
		return
	}
	for _, sym := range req.Symbols {
		for _, st := range req.Types {
			delete(s.subs, subKey{sym, category(req.Category), st})
		}
	}
}

// resubscribe replays the remembered subscription set after a reconnect,
// grouping keys by their options into one subscribe per group. It holds
// subMu across every call so a concurrent Unsubscribe cannot interleave.
func (s *Stream) resubscribe() {
	s.subMu.Lock()
	defer s.subMu.Unlock()
	if len(s.subs) == 0 {
		return
	}
	type group struct {
		cat  Category
		opts subscription
	}
	byGroup := map[group]map[SubType][]string{}
	for k, v := range s.subs {
		g := group{k.category, v}
		if byGroup[g] == nil {
			byGroup[g] = map[SubType][]string{}
		}
		byGroup[g][k.subType] = append(byGroup[g][k.subType], k.symbol)
	}

	// The callback carries no context, so a fresh bounded one is the only
	// option.
	ctx, cancel := context.WithTimeout(context.Background(), s.opts.ConnectTimeout)
	defer cancel()
	for g, byType := range byGroup {
		for st, syms := range byType {
			req := SubscribeRequest{
				Symbols: syms, Category: g.cat, Types: []SubType{st},
				Depth: g.opts.depth, Snapshot: g.opts.snapshot, Overnight: g.opts.overnight,
			}
			// A resubscribe that fails is retried by the next reconnect,
			// which replays the same set; the failure is visible through
			// Dropped so a persistent one is not silent.
			if err := s.client.subscribe(ctx, s.session, req); err != nil {
				s.dropped.Add(1)
			}
		}
	}
}

// Close ends the stream and disconnects from the broker. It is safe to call
// more than once.
func (s *Stream) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
		if s.conn != nil {
			s.conn.Disconnect(250)
		}
	})
	return nil
}

// newSessionID returns a random hex session identifier.
func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on any supported platform; a colliding
		// all-zero session id, bound wrong at subscribe, is worse than a
		// loud failure. This matches internal/signing's policy.
		panic("webull: cannot read random bytes for session id: " + err.Error())
	}
	return fmt.Sprintf("%x", b)
}
