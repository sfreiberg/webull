package streaming

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/sfreiberg/webull/internal/transport"
)

// SubType is a category of streamed data to subscribe to.
type SubType string

// Subscription data types.
const (
	SubQuote    SubType = "QUOTE"
	SubSnapshot SubType = "SNAPSHOT"
	SubTick     SubType = "TICK"
)

// ErrSubscribeFailed wraps an error from the subscribe or unsubscribe HTTP
// call. Authentication and entitlement are enforced there — the signed
// subscribe call is the real security boundary — not at the MQTT
// connection, which the broker accepts for a well-formed app key and which
// paho reports as connected even when the broker later rejects a bad key
// (paho maps Webull's non-standard CONNACK codes to a nil error). A bad
// key therefore surfaces here, when the first Subscribe is refused.
var ErrSubscribeFailed = errors.New("streaming: subscribe failed")

// Client opens the MQTT market-data stream. It is safe for concurrent use;
// each Connect call opens its own connection.
type Client struct {
	doer *transport.Doer
	// httpHost serves the subscribe and unsubscribe calls.
	httpHost string
	// brokerHost and brokerPort address the MQTT broker.
	brokerHost string
	brokerPort int

	// newClient builds the paho client; tests replace it with a fake.
	newClient func(mqtt.ClientOptions) mqtt.Client
}

// New returns a Client that serves subscribe/unsubscribe over httpHost and
// streams from the MQTT broker at brokerHost.
//
// It exists so the root webull package can compose a Client; callers of the
// SDK should use webull.NewClient.
func New(doer *transport.Doer, httpHost, brokerHost string) *Client {
	return &Client{
		doer:       doer,
		httpHost:   httpHost,
		brokerHost: brokerHost,
		brokerPort: 1883, // TLS over TCP; 8883 is MQTT-over-WebSocket
		newClient:  func(o mqtt.ClientOptions) mqtt.Client { return mqtt.NewClient(&o) },
	}
}

// ConnectOptions tunes a streaming connection.
type ConnectOptions struct {
	// SessionID identifies the connection to the subscribe endpoint; empty
	// generates a random one. Reuse across connections is not allowed.
	SessionID string
	// ConnectTimeout bounds the initial broker handshake; zero means ten
	// seconds.
	ConnectTimeout time.Duration
	// ReconnectDelay is the pause between reconnect attempts; zero means
	// five seconds.
	ReconnectDelay time.Duration
	// QueueSize bounds how many received messages buffer before Recv is
	// called; zero means 256. A full queue drops the oldest message.
	QueueSize int
}

// Connect opens the broker connection and blocks until it is established,
// so a returned Stream is live. Nothing flows until Subscribe is called.
func (c *Client) Connect(ctx context.Context, opts ...ConnectOptions) (*Stream, error) {
	var o ConnectOptions
	if len(opts) > 0 {
		o = opts[0]
	}
	if o.SessionID == "" {
		o.SessionID = newSessionID()
	}
	if o.ConnectTimeout <= 0 {
		o.ConnectTimeout = 10 * time.Second
	}
	if o.ReconnectDelay <= 0 {
		o.ReconnectDelay = 5 * time.Second
	}
	if o.QueueSize <= 0 {
		o.QueueSize = 256
	}

	s := &Stream{
		client:  c,
		session: o.SessionID,
		opts:    o,
		queue:   make(chan queued, o.QueueSize),
		subs:    map[subKey]subscription{},
	}
	if err := s.connect(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// SessionID returns the session identifier the stream connected with.
func (s *Stream) SessionID() string { return s.session }

// SubscribeRequest selects instruments and data types to stream.
type SubscribeRequest struct {
	// Symbols holds up to 100 symbols.
	Symbols []string
	// Category defaults to USStock.
	Category Category
	// Types selects the data types; at least one is required.
	Types []SubType
	// Depth sets the LV2 book depth; zero uses the server default of 10.
	Depth int
	// Snapshot requests an immediate snapshot on subscribe.
	Snapshot bool
	// Overnight includes the overnight session (US stocks only).
	Overnight bool
}

// Subscribe registers interest over HTTP; matching messages then arrive via
// Recv. It requires a live connection.
func (s *Stream) Subscribe(ctx context.Context, req SubscribeRequest) error {
	if len(req.Symbols) == 0 || len(req.Types) == 0 {
		return errors.New("streaming: Subscribe requires Symbols and Types")
	}
	if err := s.client.subscribe(ctx, s.session, req); err != nil {
		return err
	}
	s.remember(req)
	return nil
}

func (c *Client) subscribe(ctx context.Context, session string, req SubscribeRequest) error {
	body := map[string]any{
		"session_id": session,
		"symbols":    req.Symbols,
		"category":   string(category(req.Category)),
		"sub_types":  subTypeStrings(req.Types),
		"grab":       boolString(req.Snapshot),
	}
	if req.Depth > 0 {
		body["depth"] = fmt.Sprintf("%d", req.Depth)
	}
	if req.Overnight {
		body["overnight_required"] = true
	}
	if err := c.doer.Post(ctx, c.httpHost, "/market-data/streaming/subscribe", body, nil); err != nil {
		return fmt.Errorf("%w: %w", ErrSubscribeFailed, err)
	}
	return nil
}

// UnsubscribeRequest selects what to stop streaming.
type UnsubscribeRequest struct {
	Symbols  []string
	Category Category
	Types    []SubType
	// All cancels every subscription; the other fields are then ignored.
	All bool
}

// Unsubscribe stops matching messages. It requires a live connection.
func (s *Stream) Unsubscribe(ctx context.Context, req UnsubscribeRequest) error {
	body := map[string]any{"session_id": s.session}
	if req.All {
		body["unsubscribe_all"] = true
	} else {
		if len(req.Symbols) == 0 || len(req.Types) == 0 {
			return errors.New("streaming: Unsubscribe requires Symbols and Types unless All is set")
		}
		body["symbols"] = req.Symbols
		body["category"] = string(category(req.Category))
		body["sub_types"] = subTypeStrings(req.Types)
	}
	if err := s.client.doer.Post(ctx, s.client.httpHost, "/market-data/streaming/unsubscribe", body, nil); err != nil {
		return fmt.Errorf("%w: %w", ErrSubscribeFailed, err)
	}
	s.forget(req)
	return nil
}

// category returns c, or USStock when unset.
func category(c Category) Category {
	if c == "" {
		return USStock
	}
	return c
}

func subTypeStrings(ts []SubType) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = string(t)
	}
	return out
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// tlsConfig is the broker TLS configuration.
func tlsConfig() *tls.Config { return &tls.Config{MinVersion: tls.VersionTLS12} }
