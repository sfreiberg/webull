// Package events streams real-time trade events from Webull over gRPC:
// order status changes, position changes and option events.
//
// Obtain a Client from the root package and subscribe:
//
//	stream, err := client.Events.Subscribe(ctx, events.SubscribeRequest{AccountIDs: []string{acctID}})
//	defer stream.Close()
//	for {
//		ev, err := stream.Recv()
//		if err != nil {
//			break
//		}
//		if ev.Kind == events.KindOrder && ev.Order != nil {
//			fmt.Println(ev.Order.Scene, ev.Order.Symbol)
//		}
//	}
//
// Subscribe blocks until the server acknowledges the subscription, so a
// returned stream is live. Recv handles heartbeats internally and
// reconnects on transient failures with a fixed delay, resubscribing to the
// same accounts; an expired subscription is renewed the same way. An
// authentication rejection or the server's connection limit are terminal
// and surface as ErrAuthFailed and ErrConnectionLimit.
//
// After a reconnect Webull may redeliver an event already seen. Each event
// carries a RequestID for callers that need to deduplicate.
package events

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/sfreiberg/webull/internal/wire"
)

// Time is a point in time in Webull's ISO 8601 form. It is an alias of the
// shared wire type also used by the marketdata package.
type Time = wire.Time

// NullDecimal is the SDK's decimal type: it embeds shopspring's and also
// decodes Webull's absent forms, null and an empty string.
type NullDecimal = wire.NullDecimal

// Terminal stream errors.
var (
	// ErrAuthFailed is returned when the server rejects the stream's
	// credentials. Retrying would fail the same way.
	ErrAuthFailed = errors.New("events: authentication rejected")

	// ErrConnectionLimit is returned when the server refuses the stream
	// because too many connections are open for the key. Reconnecting
	// would make that worse, so the stream stops instead.
	ErrConnectionLimit = errors.New("events: connection limit exceeded")
)

// SubscriptionType selects an event family to subscribe to. Values combine
// as a bitmask on the wire.
type SubscriptionType uint32

// Subscription types.
const (
	Orders    SubscriptionType = 1
	Positions SubscriptionType = 2
	Options   SubscriptionType = 4
)

// Kind identifies the family of a received business event, as Webull tags
// it on the wire.
type Kind uint32

// Event kinds.
const (
	KindOrder    Kind = 1024
	KindPosition Kind = 1028
	KindOption   Kind = 1032
)

// Event is one business event from the stream.
type Event struct {
	Kind Kind
	// Order is populated for KindOrder events whose JSON payload decodes;
	// check it for nil, since a payload in an unexpected shape leaves it
	// unset with the raw bytes still in Payload.
	Order *OrderEvent
	// Position is populated for KindPosition events carrying a JSON
	// payload. Webull documents only the event-contract settlement shape,
	// so fields outside it are visible only through Payload.
	Position *PositionEvent
	// Payload is the verbatim payload for every event, including kinds
	// this package does not model.
	Payload []byte
	// RequestID identifies the delivery; after a reconnect a redelivered
	// event carries the same one.
	RequestID string
	Time      time.Time
}

// SceneType is what happened to an order.
type SceneType string

// Order event scenes.
const (
	Filled        SceneType = "FILLED"
	FinalFilled   SceneType = "FINAL_FILLED"
	PlaceFailed   SceneType = "PLACE_FAILED"
	ModifySuccess SceneType = "MODIFY_SUCCESS"
	ModifyFailed  SceneType = "MODIFY_FAILED"
	CancelSuccess SceneType = "CANCEL_SUCCESS"
	CancelFailed  SceneType = "CANCEL_FAILED"
)

// OrderEvent is an order status change.
type OrderEvent struct {
	RequestID      string      `json:"request_id"`
	AccountID      string      `json:"account_id"`
	ClientOrderID  string      `json:"client_order_id"`
	OrderID        string      `json:"order_id"`
	InstrumentID   string      `json:"instrument_id"`
	Symbol         string      `json:"symbol"`
	Category       string      `json:"category"`
	Status         string      `json:"order_status"`
	Scene          SceneType   `json:"scene_type"`
	Side           string      `json:"side"`
	OrderType      string      `json:"order_type"`
	Quantity       NullDecimal `json:"qty"`
	FilledQuantity NullDecimal `json:"filled_qty"`
	FilledPrice    NullDecimal `json:"filled_price"`
	FilledTime     Time        `json:"filled_time"`
}

// PositionEvent is a position change. Webull documents only the
// event-contract settlement form; every field may be absent for other
// instruments, whose full payload is in Event.Payload.
type PositionEvent struct {
	EventName    string      `json:"event_name"`
	YesCondition string      `json:"yes_condition"`
	SettleResult string      `json:"settle_result"`
	SettleSide   string      `json:"settle_side"`
	Quantity     NullDecimal `json:"quantity"`
	Cost         NullDecimal `json:"cost"`
	SettleAmount NullDecimal `json:"settle_amount"`
}

// jsonContentType is the payload content type carrying JSON. Matching is
// by prefix so that a parameterised form such as
// "application/json;charset=UTF-8" also decodes.
const jsonContentType = "application/json"

// decodeEvent builds an Event from a business response.
func decodeEvent(kind Kind, contentType string, payload []byte, requestID string, ts int64) *Event {
	ev := &Event{
		Kind:      kind,
		Payload:   payload,
		RequestID: requestID,
	}
	if ts != 0 {
		ev.Time = time.UnixMilli(ts).UTC()
	}
	if !strings.HasPrefix(contentType, jsonContentType) {
		return ev
	}
	switch kind {
	case KindOrder:
		var o OrderEvent
		if err := json.Unmarshal(payload, &o); err == nil {
			ev.Order = &o
		}
	case KindPosition:
		var p PositionEvent
		if err := json.Unmarshal(payload, &p); err == nil {
			ev.Position = &p
		}
	}
	return ev
}
