package events

import "testing"

// Event payloads arrive from the server as opaque bytes with a
// server-controlled content type. Decoding must always produce an Event —
// unparseable payloads stay raw on Event.Payload — and never panic.
func FuzzDecodeEvent(f *testing.F) {
	f.Add(uint32(KindOrder), "application/json", []byte(`{"order_id":"1"}`), "req-1", int64(1725264000000))
	f.Add(uint32(KindOrder), "application/json;charset=UTF-8", []byte(`{`), "", int64(0))
	f.Add(uint32(KindPosition), "text/plain", []byte("ping"), "r", int64(-1))
	f.Add(uint32(KindOption), "application/json", []byte(`[]`), "", int64(1))
	f.Add(uint32(999), "", []byte{0xff, 0xfe}, "", int64(0))
	f.Fuzz(func(t *testing.T, kind uint32, contentType string, payload []byte, requestID string, ts int64) {
		ev := decodeEvent(Kind(kind), contentType, payload, requestID, ts)
		if ev == nil {
			t.Error("decodeEvent must always return an event")
		}
	})
}
