package wire

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

// TestDecimalToleratesEmptyString is the regression test for the bug where
// Webull returns "" for an absent decimal and shopspring's own types reject
// it. Both wire types must decode "" (and null, and a real value) without
// error.
func TestDecimalToleratesEmptyString(t *testing.T) {
	t.Run("Decimal", func(t *testing.T) {
		cases := map[string]struct {
			in       string
			wantZero bool
			wantVal  string
		}{
			"empty string": {`""`, true, ""},
			"null":         {`null`, true, ""},
			"value":        {`"231.42"`, false, "231.42"},
			"number":       {`231.42`, false, "231.42"},
		}
		for name, tc := range cases {
			var d Decimal
			if err := json.Unmarshal([]byte(tc.in), &d); err != nil {
				t.Errorf("%s: unexpected error: %v", name, err)
				continue
			}
			if tc.wantZero && !d.IsZero() {
				t.Errorf("%s: got %v, want zero", name, d)
			}
			if !tc.wantZero && !d.Equal(decimal.RequireFromString(tc.wantVal)) {
				t.Errorf("%s: got %v, want %s", name, d, tc.wantVal)
			}
		}
	})

	t.Run("NullDecimal", func(t *testing.T) {
		for name, in := range map[string]string{"empty string": `""`, "null": `null`} {
			var n NullDecimal
			if err := json.Unmarshal([]byte(in), &n); err != nil {
				t.Errorf("%s: unexpected error: %v", name, err)
			}
			if n.Valid {
				t.Errorf("%s: an absent decimal must be invalid, got valid=%v", name, n.Valid)
			}
		}
		var n NullDecimal
		if err := json.Unmarshal([]byte(`"231.42"`), &n); err != nil {
			t.Fatal(err)
		}
		if !n.Valid || !n.Decimal.Equal(decimal.RequireFromString("231.42")) {
			t.Errorf("a real value must decode present: %+v", n)
		}
	})
}

// TestDecimalRejectsGarbage confirms the tolerant decode does not swallow a
// genuinely malformed value.
func TestDecimalRejectsGarbage(t *testing.T) {
	var d Decimal
	if err := json.Unmarshal([]byte(`"not a number"`), &d); err == nil {
		t.Error("Decimal must reject a non-numeric string")
	}
	var n NullDecimal
	if err := json.Unmarshal([]byte(`"not a number"`), &n); err == nil {
		t.Error("NullDecimal must reject a non-numeric string")
	}
}

// TestDecimalDecodesInStruct proves the types work as struct fields, the way
// the service packages use them — including "" for one field and a value for
// another in the same object.
func TestDecimalDecodesInStruct(t *testing.T) {
	var v struct {
		Price Decimal     `json:"price"`
		Delta NullDecimal `json:"delta"`
		Gamma NullDecimal `json:"gamma"`
	}
	// price present, delta absent as "", gamma absent as null.
	if err := json.Unmarshal([]byte(`{"price":"180.00","delta":"","gamma":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if !v.Price.Equal(decimal.RequireFromString("180.00")) {
		t.Errorf("price = %v", v.Price)
	}
	if v.Delta.Valid || v.Gamma.Valid {
		t.Errorf("empty-string and null greeks must both be absent: delta=%+v gamma=%+v", v.Delta, v.Gamma)
	}
}

// TestNullDecimalMarshalRoundTrip proves request marshaling is unaffected:
// a present value serializes as a bare decimal string, and an absent one is
// omitted under omitzero exactly as shopspring's type is.
func TestNullDecimalMarshalRoundTrip(t *testing.T) {
	type req struct {
		Qty   NullDecimal `json:"qty,omitzero"`
		Price NullDecimal `json:"price,omitzero"`
	}
	b, err := json.Marshal(req{Price: RequireNullDecimal("5")})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"price":"5"}` {
		t.Errorf("marshaled = %s, want {\"price\":\"5\"}", b)
	}
}
