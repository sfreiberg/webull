package wire

import (
	"encoding/json"
	"testing"
)

// The wire types absorb whatever shape Webull sends. Fuzzing pins the
// contract that hostile or malformed input produces an error or a zero
// value — never a panic, and never a silently wrong non-zero value from
// unparseable input.

func FuzzTimeUnmarshalJSON(f *testing.F) {
	for _, seed := range []string{
		`"2026-09-02T12:00:00.000+0000"`, `"2026-09-02T12:00:00Z"`,
		`"2026-09-02"`, `"20260902"`, `""`, `null`, `"not a time"`,
		`123`, `{}`, `"2026-13-45T99:99:99Z"`, `"0001-01-01"`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var v Time
		if err := v.UnmarshalJSON(data); err != nil && !v.IsZero() {
			t.Errorf("error with non-zero value: %v -> %v", string(data), v)
		}
	})
}

func FuzzDecimalUnmarshalJSON(f *testing.F) {
	for _, seed := range []string{
		`"1.50"`, `"-0.001"`, `"1.78E8"`, `1.5`, `""`, `null`,
		`"NaN"`, `"Inf"`, `"1e999999"`, `"--1"`, `"0x10"`, `[]`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var d Decimal
		_ = d.UnmarshalJSON(data)
		var n NullDecimal
		if err := n.UnmarshalJSON(data); err == nil {
			// A successful decode must round-trip through Marshal without
			// panicking.
			_, _ = json.Marshal(n)
		}
	})
}
