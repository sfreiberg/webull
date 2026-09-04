package streaming

import "testing"

// The broker's payloads cross a trust boundary: whatever arrives on the wire
// must decode to a Message, an error, or the echo topic's nil — never a
// panic that would take the caller's process down.
func FuzzDecode(f *testing.F) {
	topics := []string{"echo", string(TypeSnapshot), string(TypeQuote), string(TypeTick),
		string(TypeEventTick), string(TypeEventSnapshot), string(TypeEventQuote), "unknown-topic"}
	for _, topic := range topics {
		f.Add(topic, []byte{})
		f.Add(topic, []byte{0x08, 0x96, 0x01})
		f.Add(topic, []byte("not a protobuf"))
	}
	f.Fuzz(func(t *testing.T, topic string, payload []byte) {
		msg, err := decode(topic, payload)
		if msg != nil && err != nil {
			t.Errorf("both a message and an error for topic %q", topic)
		}
	})
}
