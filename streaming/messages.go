// Package streaming delivers real-time market data from Webull over MQTT:
// snapshots, top-of-book and depth quotes, and ticks for stocks and event
// contracts.
//
// Obtain a Client from the root package, connect, and subscribe:
//
//	stream, err := client.Streaming.Connect(ctx)
//	defer stream.Close()
//	err = stream.Subscribe(ctx, streaming.SubscribeRequest{
//		Symbols: []string{"AAPL"},
//		Types:   []streaming.SubType{streaming.SubSnapshot},
//	})
//	for {
//		msg, err := stream.Recv(ctx)
//		if err != nil {
//			break
//		}
//		if msg.Type == streaming.TypeSnapshot {
//			fmt.Println(msg.Snapshot.Symbol, msg.Snapshot.Price)
//		}
//	}
//
// The MQTT connection authenticates with the app key; what flows over it is
// governed by Subscribe and Unsubscribe, which are HTTP calls binding the
// connection's session. On a reconnect the stream resubscribes to
// everything it was subscribed to.
package streaming

import (
	"strconv"
	"time"

	"github.com/shopspring/decimal"
	"google.golang.org/protobuf/proto"

	"github.com/sfreiberg/webull/internal/streampb"
	"github.com/sfreiberg/webull/internal/wire"
)

// Category identifies an asset class. It is an alias of the shared wire
// type also used by the marketdata package.
type Category = wire.Category

// Categories the streaming subscribe endpoint documents.
const (
	USStock Category = "US_STOCK"
	USETF   Category = "US_ETF"
)

// MessageType is the topic a streamed message arrived on, which names its
// payload type.
type MessageType string

// Message types.
const (
	TypeQuote         MessageType = "quote"
	TypeSnapshot      MessageType = "snapshot"
	TypeTick          MessageType = "tick"
	TypeEventQuote    MessageType = "event-quote"
	TypeEventSnapshot MessageType = "event-snapshot"
	TypeEventTick     MessageType = "event-tick"
	TypeNotice        MessageType = "notice"
)

// Message is one streamed market-data message. Exactly one of the typed
// fields matching Type is populated; a topic this package does not model
// arrives with only Type and Raw set.
type Message struct {
	Type          MessageType
	Snapshot      *Snapshot
	Quote         *Quote
	Tick          *Tick
	EventSnapshot *EventSnapshot
	EventQuote    *EventQuote
	EventTick     *EventTick
	// Notice carries the text of a notice message.
	Notice string
	// Raw is the verbatim payload for unmodeled topics.
	Raw []byte
}

// Basic identifies the instrument a message concerns.
type Basic struct {
	Symbol       string
	InstrumentID string
	Time         time.Time
	// Session is the trading session the update belongs to.
	Session string
}

// Snapshot is the current state of an instrument, with extended-hours and
// overnight sub-sessions where subscribed.
type Snapshot struct {
	Basic
	TradeTime   time.Time
	Price       decimal.NullDecimal
	Open        decimal.NullDecimal
	High        decimal.NullDecimal
	Low         decimal.NullDecimal
	PreClose    decimal.NullDecimal
	Volume      decimal.NullDecimal
	Change      decimal.NullDecimal
	ChangeRatio decimal.NullDecimal

	ExtendedTradeTime   time.Time
	ExtendedPrice       decimal.NullDecimal
	ExtendedHigh        decimal.NullDecimal
	ExtendedLow         decimal.NullDecimal
	ExtendedVolume      decimal.NullDecimal
	ExtendedChange      decimal.NullDecimal
	ExtendedChangeRatio decimal.NullDecimal

	OvernightTradeTime   time.Time
	OvernightPrice       decimal.NullDecimal
	OvernightHigh        decimal.NullDecimal
	OvernightLow         decimal.NullDecimal
	OvernightVolume      decimal.NullDecimal
	OvernightChange      decimal.NullDecimal
	OvernightChangeRatio decimal.NullDecimal
}

// Level is one price level of a streamed book. Orders and Brokers carry
// market-participant detail at entitled depths.
type Level struct {
	Price   decimal.NullDecimal
	Size    decimal.NullDecimal
	Orders  []LevelOrder
	Brokers []LevelBroker
}

// LevelOrder is one market participant's size at a level.
type LevelOrder struct {
	MPID string
	Size decimal.NullDecimal
}

// LevelBroker identifies a broker at a level.
type LevelBroker struct {
	ID   string
	Name string
}

// Quote is an order-book update.
type Quote struct {
	Basic
	Asks []Level
	Bids []Level
}

// Tick is one trade.
type Tick struct {
	Basic
	Time   time.Time
	Price  decimal.NullDecimal
	Volume decimal.NullDecimal
	Side   string
}

// EventSnapshot is the current state of an event-contract market.
type EventSnapshot struct {
	Basic
	Price         decimal.NullDecimal
	Volume        decimal.NullDecimal
	LastTradeTime time.Time
	OpenInterest  decimal.NullDecimal
	YesAsk        decimal.NullDecimal
	YesBid        decimal.NullDecimal
	YesAskSize    decimal.NullDecimal
	YesBidSize    decimal.NullDecimal
	NoAsk         decimal.NullDecimal
	NoBid         decimal.NullDecimal
	NoAskSize     decimal.NullDecimal
	NoBidSize     decimal.NullDecimal
}

// EventLevel is one price level of an event contract's book.
type EventLevel struct {
	Price decimal.NullDecimal
	Size  decimal.NullDecimal
}

// EventQuote is an event-contract book update, one side per outcome.
type EventQuote struct {
	Basic
	YesBids []EventLevel
	NoBids  []EventLevel
}

// EventTick is one event-contract trade.
type EventTick struct {
	Basic
	YesPrice decimal.NullDecimal
	NoPrice  decimal.NullDecimal
	Volume   decimal.NullDecimal
	Side     string
	TradeID  string
	Time     time.Time
}

// dec parses a wire decimal string; empty means absent.
func dec(s string) decimal.NullDecimal {
	if s == "" {
		return decimal.NullDecimal{}
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.NullDecimal{}
	}
	return decimal.NullDecimal{Decimal: d, Valid: true}
}

// millis parses an epoch-millisecond string; empty or zero means absent.
func millis(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n == 0 {
		return time.Time{}
	}
	return time.UnixMilli(n).UTC()
}

func basic(b *streampb.Basic) Basic {
	if b == nil {
		return Basic{}
	}
	return Basic{
		Symbol:       b.GetSymbol(),
		InstrumentID: b.GetInstrumentId(),
		Time:         millis(b.GetTimestamp()),
		Session:      b.GetTradingSession(),
	}
}

func levels(in []*streampb.AskBid) []Level {
	out := make([]Level, 0, len(in))
	for _, l := range in {
		lv := Level{Price: dec(l.GetPrice()), Size: dec(l.GetSize())}
		for _, o := range l.GetOrder() {
			lv.Orders = append(lv.Orders, LevelOrder{MPID: o.GetMpid(), Size: dec(o.GetSize())})
		}
		for _, b := range l.GetBroker() {
			lv.Brokers = append(lv.Brokers, LevelBroker{ID: b.GetBid(), Name: b.GetName()})
		}
		out = append(out, lv)
	}
	return out
}

func eventLevels(in []*streampb.EventAskBid) []EventLevel {
	out := make([]EventLevel, 0, len(in))
	for _, l := range in {
		out = append(out, EventLevel{Price: dec(l.GetPrice()), Size: dec(l.GetSize())})
	}
	return out
}

// decode converts one MQTT message into a Message. It returns nil for the
// broker's echo topic, which carries nothing for the caller.
func decode(topic string, payload []byte) (*Message, error) {
	switch MessageType(topic) {
	case "echo":
		return nil, nil
	case TypeSnapshot:
		var pb streampb.Snapshot
		if err := proto.Unmarshal(payload, &pb); err != nil {
			return nil, err
		}
		return &Message{Type: TypeSnapshot, Snapshot: &Snapshot{
			Basic:     basic(pb.GetBasic()),
			TradeTime: millis(pb.GetTradeTime()),
			Price:     dec(pb.GetPrice()), Open: dec(pb.GetOpen()), High: dec(pb.GetHigh()), Low: dec(pb.GetLow()),
			PreClose: dec(pb.GetPreClose()), Volume: dec(pb.GetVolume()),
			Change: dec(pb.GetChange()), ChangeRatio: dec(pb.GetChangeRatio()),
			ExtendedTradeTime: millis(pb.GetExtTradeTime()),
			ExtendedPrice:     dec(pb.GetExtPrice()), ExtendedHigh: dec(pb.GetExtHigh()), ExtendedLow: dec(pb.GetExtLow()),
			ExtendedVolume: dec(pb.GetExtVolume()), ExtendedChange: dec(pb.GetExtChange()), ExtendedChangeRatio: dec(pb.GetExtChangeRatio()),
			OvernightTradeTime: millis(pb.GetOvnTradeTime()),
			OvernightPrice:     dec(pb.GetOvnPrice()), OvernightHigh: dec(pb.GetOvnHigh()), OvernightLow: dec(pb.GetOvnLow()),
			OvernightVolume: dec(pb.GetOvnVolume()), OvernightChange: dec(pb.GetOvnChange()), OvernightChangeRatio: dec(pb.GetOvnChangeRatio()),
		}}, nil
	case TypeQuote:
		var pb streampb.Quote
		if err := proto.Unmarshal(payload, &pb); err != nil {
			return nil, err
		}
		return &Message{Type: TypeQuote, Quote: &Quote{
			Basic: basic(pb.GetBasic()),
			Asks:  levels(pb.GetAsks()),
			Bids:  levels(pb.GetBids()),
		}}, nil
	case TypeTick:
		var pb streampb.Tick
		if err := proto.Unmarshal(payload, &pb); err != nil {
			return nil, err
		}
		return &Message{Type: TypeTick, Tick: &Tick{
			Basic: basic(pb.GetBasic()),
			Time:  millis(pb.GetTime()),
			Price: dec(pb.GetPrice()), Volume: dec(pb.GetVolume()), Side: pb.GetSide(),
		}}, nil
	case TypeEventSnapshot:
		var pb streampb.EventSnapshot
		if err := proto.Unmarshal(payload, &pb); err != nil {
			return nil, err
		}
		return &Message{Type: TypeEventSnapshot, EventSnapshot: &EventSnapshot{
			Basic: basic(pb.GetBasic()),
			Price: dec(pb.GetPrice()), Volume: dec(pb.GetVolume()),
			LastTradeTime: millis(pb.GetLastTradeTime()),
			OpenInterest:  dec(pb.GetOpenInterest()),
			YesAsk:        dec(pb.GetYesAsk()), YesBid: dec(pb.GetYesBid()),
			YesAskSize: dec(pb.GetYesAskSize()), YesBidSize: dec(pb.GetYesBidSize()),
			NoAsk: dec(pb.GetNoAsk()), NoBid: dec(pb.GetNoBid()),
			NoAskSize: dec(pb.GetNoAskSize()), NoBidSize: dec(pb.GetNoBidSize()),
		}}, nil
	case TypeEventQuote:
		var pb streampb.EventQuote
		if err := proto.Unmarshal(payload, &pb); err != nil {
			return nil, err
		}
		return &Message{Type: TypeEventQuote, EventQuote: &EventQuote{
			Basic:   basic(pb.GetBasic()),
			YesBids: eventLevels(pb.GetYesBids()),
			NoBids:  eventLevels(pb.GetNoBids()),
		}}, nil
	case TypeEventTick:
		var pb streampb.EventTick
		if err := proto.Unmarshal(payload, &pb); err != nil {
			return nil, err
		}
		return &Message{Type: TypeEventTick, EventTick: &EventTick{
			Basic:    basic(pb.GetBasic()),
			YesPrice: dec(pb.GetYesPrice()), NoPrice: dec(pb.GetNoPrice()),
			Volume: dec(pb.GetVolume()), Side: pb.GetSide(), TradeID: pb.GetTradeId(),
			Time: millis(pb.GetTime()),
		}}, nil
	case TypeNotice:
		return &Message{Type: TypeNotice, Notice: string(payload), Raw: payload}, nil
	}
	return &Message{Type: MessageType(topic), Raw: payload}, nil
}
