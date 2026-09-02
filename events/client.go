package events

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
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

	// DialOptions, when non-nil, replaces the default dial options (TLS
	// with system roots, and keepalive pings that detect a half-open
	// connection). Use it to route through a proxy, trust a private CA,
	// or reach a plaintext test server. Set it before the first
	// Subscribe; it must not change afterwards.
	DialOptions []grpc.DialOption
}

// New returns a Client that connects to host with signer's credentials.
// Host may carry an explicit port; without one, 443 is used.
//
// It exists so the root webull package can compose a Client; callers of the
// SDK should use webull.NewClient.
func New(signer *signing.Signer, host string) *Client {
	return &Client{signer: signer, host: host}
}

// target returns the dial target, appending the default port only when the
// host does not already carry one.
func (c *Client) target() string {
	if strings.Contains(c.host, ":") {
		return c.host
	}
	return c.host + ":443"
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
	// means unlimited. A reconnect that reaches acknowledgement resets
	// the count. A renewal after SubscribeExpired is not charged.
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
	if err := s.connect(ctx); err != nil {
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

	// mu guards conn, which Close may tear down concurrently with a
	// reconnect running under Recv.
	mu   sync.Mutex
	conn *grpc.ClientConn
	rpc  grpc.ServerStreamingClient[eventspb.SubscribeResponse]
}

// connect dials, subscribes, and consumes the acknowledgement.
func (s *Stream) connect(ctx context.Context) error {
	opts := s.client.DialOptions
	if opts == nil {
		opts = []grpc.DialOption{
			grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})),
			// Without keepalive a half-open connection (a NAT timeout, a
			// server gone without RST) blocks Recv forever with no error
			// for the reconnect logic to act on.
			grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: time.Minute, Timeout: 20 * time.Second}),
		}
	}
	conn, err := grpc.NewClient(s.client.target(), opts...)
	if err != nil {
		return fmt.Errorf("events: dial: %w", err)
	}

	pbReq := &eventspb.SubscribeRequest{
		SubscribeType: s.req.mask(),
		Timestamp:     time.Now().UnixMilli(),
		Accounts:      s.req.AccountIDs,
	}
	// The signature covers the serialized message bytes. gRPC marshals the
	// message again when sending, which is safe here: both marshals run in
	// the same binary over a message with no maps or unknown fields, so
	// the bytes cannot diverge.
	body, err := proto.Marshal(pbReq)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("events: marshal request: %w", err)
	}
	md := metadata.New(s.client.signer.SignStream(body))

	rpc, err := eventspb.NewEventServiceClient(conn).Subscribe(metadata.NewOutgoingContext(ctx, md), pbReq)
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

	// Publish under the lock, re-checking for a concurrent Close: without
	// the check, a Close that ran while the acknowledgement was in flight
	// would leave this fresh connection — a live subscription counting
	// against the key's connection limit — leaked with nobody to close it.
	s.mu.Lock()
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		_ = conn.Close()
		return s.ctx.Err()
	}
	s.conn, s.rpc = conn, rpc
	s.mu.Unlock()
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

// retryable reports whether a stream error is worth a reconnect: a clean
// server-side end of stream (a long-lived stream being rotated), or the
// same transient status codes Webull's own client retries. Unknown counts
// only for a genuine gRPC status, so a local failure mapped to Unknown by
// status.Code does not loop.
func retryable(err error) bool {
	if errors.Is(err, io.EOF) {
		return true
	}
	switch status.Code(err) {
	case codes.Unavailable, codes.Internal:
		return true
	case codes.Unknown:
		var se interface{ GRPCStatus() *status.Status }
		return errors.As(err, &se)
	}
	return false
}

// isTerminal reports whether a connect error must end the stream rather
// than be retried.
func isTerminal(err error) bool {
	return errors.Is(err, ErrAuthFailed) || errors.Is(err, ErrConnectionLimit) ||
		errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
		!retryable(err)
}

// Recv returns the next business event. Heartbeats are consumed
// internally, transient failures reconnect after the configured delay, and
// an expired subscription is renewed immediately without being charged to
// the reconnect budget. Terminal failures — authentication rejection, the
// connection limit, context cancellation, or reconnect attempts exhausted
// — end the stream with an error.
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
			if err := s.reconnect(&failures); err != nil {
				return nil, err
			}
			continue
		}

		if err := controlError(resp); err != nil {
			return nil, err
		}
		switch resp.GetEventType() {
		case eventspb.EventType_SubscribeSuccess, eventspb.EventType_Ping:
			continue
		case eventspb.EventType_SubscribeExpired:
			if err := s.renew(&failures); err != nil {
				return nil, err
			}
			continue
		}
		return decodeEvent(
			Kind(resp.GetEventType()),
			resp.GetContentType(),
			[]byte(resp.GetPayload()),
			resp.GetRequestId(),
			resp.GetTimestamp(),
		), nil
	}
}

// renew resubscribes immediately after a SubscribeExpired: the transport
// is healthy and nothing failed, so there is no delay and no charge to the
// reconnect budget. A renewal that fails transiently falls back to the
// delayed reconnect loop.
func (s *Stream) renew(failures *int) error {
	s.dropConn()
	err := s.connect(s.ctx)
	if err == nil {
		return nil
	}
	if isTerminal(err) {
		return err
	}
	return s.reconnect(failures)
}

// reconnect retries until a subscription is acknowledged again, so an
// outage spanning several attempts is ridden out rather than ending the
// stream at the first failed dial. It stops on a terminal error, on
// context cancellation, or when the attempt budget is spent; reaching
// acknowledgement resets the budget.
func (s *Stream) reconnect(failures *int) error {
	for {
		*failures++
		if s.req.MaxReconnectAttempts > 0 && *failures > s.req.MaxReconnectAttempts {
			return fmt.Errorf("events: gave up after %d reconnect attempts", *failures-1)
		}
		s.dropConn()
		timer := time.NewTimer(s.req.delay())
		select {
		case <-s.ctx.Done():
			timer.Stop()
			return s.ctx.Err()
		case <-timer.C:
		}
		err := s.connect(s.ctx)
		if err == nil {
			*failures = 0
			return nil
		}
		if isTerminal(err) {
			return err
		}
	}
}

// dropConn closes and forgets the current connection.
func (s *Stream) dropConn() {
	s.mu.Lock()
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
	s.mu.Unlock()
}

// Close ends the stream and releases its connection. It is safe to call
// more than once and from any goroutine.
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
