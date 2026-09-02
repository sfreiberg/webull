package events

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/sfreiberg/webull/internal/eventspb"
	"github.com/sfreiberg/webull/internal/signing"
)

// Client subscribes to the trade event stream. It is safe for concurrent
// use; each Subscribe call opens its own connection.
type Client struct {
	signer *signing.Signer
	host   string

	// dialOpts replaces the TLS dial options in tests.
	dialOpts []grpc.DialOption
}

// New returns a Client that connects to host with signer's credentials.
//
// It exists so the root webull package can compose a Client; callers of the
// SDK should use webull.NewClient.
func New(signer *signing.Signer, host string) *Client {
	return &Client{signer: signer, host: host}
}

// SubscribeRequest selects the accounts and event families to stream.
type SubscribeRequest struct {
	// AccountIDs holds the accounts to receive events for.
	AccountIDs []string
	// Types selects event families; empty subscribes to all of them.
	Types []SubscriptionType
	// ReconnectDelay is the pause before a reconnect attempt; zero means
	// five seconds, matching Webull's own clients.
	ReconnectDelay time.Duration
	// MaxReconnectAttempts bounds consecutive failed reconnects; zero
	// means unlimited. A successful reconnect resets the count.
	MaxReconnectAttempts int
}

func (r SubscribeRequest) mask() uint32 {
	if len(r.Types) == 0 {
		return uint32(Orders | Positions | Options)
	}
	var m uint32
	for _, t := range r.Types {
		m |= uint32(t)
	}
	return m
}

func (r SubscribeRequest) delay() time.Duration {
	if r.ReconnectDelay <= 0 {
		return 5 * time.Second
	}
	return r.ReconnectDelay
}

// Subscribe opens the event stream and blocks until the server acknowledges
// the subscription, so a returned Stream is live. Cancel ctx or call Close
// to end it.
func (c *Client) Subscribe(ctx context.Context, req SubscribeRequest) (*Stream, error) {
	if len(req.AccountIDs) == 0 {
		return nil, errors.New("events: SubscribeRequest.AccountIDs is required")
	}
	ctx, cancel := context.WithCancel(ctx)
	s := &Stream{client: c, req: req, ctx: ctx, cancel: cancel}
	if err := s.connect(); err != nil {
		cancel()
		return nil, err
	}
	return s, nil
}

// Stream is a live event subscription. It is not safe for concurrent Recv
// calls; Close may be called from any goroutine.
type Stream struct {
	client *Client
	req    SubscribeRequest
	ctx    context.Context
	cancel context.CancelFunc

	// mu guards conn, which Close may replace concurrently with a
	// reconnect running under Recv.
	mu   sync.Mutex
	conn *grpc.ClientConn
	rpc  grpc.ServerStreamingClient[eventspb.SubscribeResponse]
}

// connect dials, subscribes, and consumes the acknowledgement.
func (s *Stream) connect() error {
	opts := s.client.dialOpts
	if opts == nil {
		opts = []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12}))}
	}
	conn, err := grpc.NewClient(s.client.host+":443", opts...)
	if err != nil {
		return fmt.Errorf("events: dial: %w", err)
	}

	pbReq := &eventspb.SubscribeRequest{
		SubscribeType: s.req.mask(),
		Timestamp:     time.Now().UnixMilli(),
		Accounts:      s.req.AccountIDs,
	}
	// The signature covers the serialized message bytes. gRPC marshals the
	// message again when sending, which is safe here: the message has no
	// maps or unknown fields, so both marshals produce identical bytes —
	// the same assumption Webull's own client makes.
	body, err := proto.Marshal(pbReq)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("events: marshal request: %w", err)
	}
	md := metadata.New(s.client.signer.SignStream(body))

	rpc, err := eventspb.NewEventServiceClient(conn).Subscribe(metadata.NewOutgoingContext(s.ctx, md), pbReq)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("events: subscribe: %w", err)
	}

	// The first response acknowledges or rejects the subscription.
	first, err := rpc.Recv()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("events: subscribe: %w", err)
	}
	if err := controlError(first); err != nil {
		_ = conn.Close()
		return err
	}
	if first.GetEventType() != eventspb.EventType_SubscribeSuccess {
		_ = conn.Close()
		return fmt.Errorf("events: unexpected first response %v", first.GetEventType())
	}

	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
	s.rpc = rpc
	return nil
}

// controlError maps a terminal control event to its error, or nil.
func controlError(resp *eventspb.SubscribeResponse) error {
	switch resp.GetEventType() {
	case eventspb.EventType_AuthError:
		return fmt.Errorf("%w: %s", ErrAuthFailed, resp.GetPayload())
	case eventspb.EventType_NumOfConnExceed:
		return fmt.Errorf("%w: %s", ErrConnectionLimit, resp.GetPayload())
	}
	return nil
}

// retryable reports whether a stream error is worth a reconnect: the same
// transient status codes Webull's own client retries.
func retryable(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.Internal, codes.Unknown:
		return true
	}
	return false
}

// Recv returns the next business event. Heartbeats are consumed
// internally, transient failures reconnect after the configured delay, and
// an expired subscription is renewed. Terminal failures — authentication
// rejection, the connection limit, context cancellation, or reconnect
// attempts exhausted — end the stream with an error.
func (s *Stream) Recv() (*Event, error) {
	failures := 0
	for {
		resp, err := s.rpc.Recv()
		if err != nil {
			if s.ctx.Err() != nil {
				return nil, s.ctx.Err()
			}
			if !retryable(err) {
				return nil, fmt.Errorf("events: stream: %w", err)
			}
			failures++
			if s.req.MaxReconnectAttempts > 0 && failures > s.req.MaxReconnectAttempts {
				return nil, fmt.Errorf("events: gave up after %d reconnect attempts: %w", failures-1, err)
			}
			if err := s.reconnect(); err != nil {
				return nil, err
			}
			continue
		}

		switch resp.GetEventType() {
		case eventspb.EventType_SubscribeSuccess, eventspb.EventType_Ping:
			continue
		case eventspb.EventType_AuthError, eventspb.EventType_NumOfConnExceed:
			return nil, controlError(resp)
		case eventspb.EventType_SubscribeExpired:
			if err := s.reconnect(); err != nil {
				return nil, err
			}
			continue
		}
		failures = 0
		return decodeEvent(
			Kind(resp.GetEventType()),
			resp.GetContentType(),
			[]byte(resp.GetPayload()),
			resp.GetRequestId(),
			resp.GetTimestamp(),
		), nil
	}
}

// reconnect closes the failed connection, waits out the delay, and opens a
// fresh subscription.
func (s *Stream) reconnect() error {
	s.mu.Lock()
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
	s.mu.Unlock()
	select {
	case <-s.ctx.Done():
		return s.ctx.Err()
	case <-time.After(s.req.delay()):
	}
	if err := s.connect(); err != nil {
		return err
	}
	return nil
}

// Close ends the stream and releases its connection. It is safe to call
// more than once.
func (s *Stream) Close() error {
	s.cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}
