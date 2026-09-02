package streaming

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Stream is a live market-data connection. Recv is not safe for concurrent
// use; Subscribe, Unsubscribe and Close are.
type Stream struct {
	client  *Client
	session string
	opts    ConnectOptions

	conn  mqtt.Client
	queue chan queued

	// mu guards subs, the set to replay on a reconnect.
	mu   sync.Mutex
	subs map[subKey]subscription

	closeOnce sync.Once
	closed    chan struct{}
}

// queued is one received message or a decode error, in arrival order.
type queued struct {
	msg *Message
	err error
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

// connect opens the broker connection and blocks until it is established.
func (s *Stream) connect(ctx context.Context) error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("ssl://%s:%d", s.client.brokerHost, s.client.brokerPort))
	opts.SetClientID(s.session)
	opts.SetUsername(s.client.doer.Signer.AppKey)
	opts.SetPassword(newSessionID()) // the broker ignores the password; the app key authenticates
	opts.SetTLSConfig(tlsConfig())
	opts.SetCleanSession(true)
	opts.SetConnectTimeout(s.opts.ConnectTimeout)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(s.opts.ReconnectDelay)
	opts.SetMaxReconnectInterval(s.opts.ReconnectDelay)
	opts.SetOrderMatters(false)

	opts.SetDefaultPublishHandler(func(_ mqtt.Client, m mqtt.Message) {
		msg, err := decode(m.Topic(), m.Payload())
		if err == nil && msg == nil {
			return // the echo topic
		}
		s.enqueue(queued{msg: msg, err: wrapDecode(m.Topic(), err)})
	})
	// On every (re)connection, replay the subscription set so a dropped
	// connection resumes streaming what it had.
	opts.SetOnConnectHandler(func(mqtt.Client) {
		s.resubscribe()
	})

	if s.closed == nil {
		s.closed = make(chan struct{})
	}
	s.conn = s.client.newClient(*opts)

	token := s.conn.Connect()
	if !waitToken(ctx, token) {
		s.conn.Disconnect(0)
		if err := ctx.Err(); err != nil {
			return err
		}
		return errors.New("streaming: connect timed out")
	}
	// token.Error() carries paho's own connect failures (a network error,
	// or a standard MQTT CONNACK refusal). Webull's non-standard codes are
	// not surfaced here — see ErrSubscribeFailed — so a well-formed key
	// always reaches this point connected.
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

// wrapDecode annotates a decode failure with its topic.
func wrapDecode(topic string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("streaming: decoding %s: %w", topic, err)
}

// Recv returns the next message. It blocks until one arrives, the context
// is cancelled, or the stream is closed.
func (s *Stream) Recv(ctx context.Context) (*Message, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.closed:
			return nil, errors.New("streaming: stream closed")
		case q := <-s.queue:
			if q.err != nil {
				return nil, q.err
			}
			return q.msg, nil
		}
	}
}

// enqueue adds a message, dropping the oldest if the queue is full so a
// slow consumer cannot block the MQTT callback.
func (s *Stream) enqueue(q queued) {
	for {
		select {
		case s.queue <- q:
			return
		default:
			select {
			case <-s.queue:
			default:
			}
		}
	}
}

// remember records a subscription for replay on reconnect.
func (s *Stream) remember(req SubscribeRequest) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
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

// resubscribe replays the remembered subscription set after a reconnect.
// It groups keys by their options and issues one subscribe per group.
func (s *Stream) resubscribe() {
	s.mu.Lock()
	if len(s.subs) == 0 {
		s.mu.Unlock()
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
	s.mu.Unlock()

	// A reconnect is driven by paho's callback with no caller context, so a
	// fresh bounded one is the only option here.
	ctx, cancel := context.WithTimeout(context.Background(), s.opts.ConnectTimeout) //nolint:contextcheck // callback has no context
	defer cancel()
	for g, byType := range byGroup {
		for st, syms := range byType {
			req := SubscribeRequest{
				Symbols: syms, Category: g.cat, Types: []SubType{st},
				Depth: g.opts.depth, Snapshot: g.opts.snapshot, Overnight: g.opts.overnight,
			}
			if err := s.client.subscribe(ctx, s.session, req); err != nil {
				s.enqueue(queued{err: fmt.Errorf("streaming: resubscribe: %w", err)})
			}
		}
	}
}

// Close ends the stream and disconnects from the broker. It is safe to call
// more than once.
func (s *Stream) Close() error {
	s.closeOnce.Do(func() {
		if s.closed != nil {
			close(s.closed)
		}
		if s.conn != nil {
			s.conn.Disconnect(250)
		}
	})
	return nil
}

// newSessionID returns a random hex session identifier.
func newSessionID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}
