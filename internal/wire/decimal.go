package wire

import (
	"bytes"

	"github.com/shopspring/decimal"
)

// Decimal is a fixed-point decimal for JSON fields. It behaves exactly like
// github.com/shopspring/decimal.Decimal, which it embeds, except that it
// decodes Webull's absent forms — JSON null and an empty string — to zero.
// Shopspring's own type rejects the empty string with an error, and Webull
// returns "" for an absent numeric field (an option greek outside market
// hours, for example) rather than null or omitting it.
type Decimal struct{ decimal.Decimal }

// UnmarshalJSON decodes a JSON number or numeric string, treating null and
// an empty string as zero.
func (d *Decimal) UnmarshalJSON(b []byte) error {
	if isJSONBlank(b) {
		d.Decimal = decimal.Decimal{}
		return nil
	}
	return d.Decimal.UnmarshalJSON(b)
}

// NullDecimal is a decimal that may be absent, for JSON fields. It behaves
// exactly like github.com/shopspring/decimal.NullDecimal, which it embeds,
// except that it decodes both null and an empty string to an absent
// (Valid == false) value. Shopspring's own type rejects the empty string.
type NullDecimal struct{ decimal.NullDecimal }

// UnmarshalJSON decodes a JSON number or numeric string, treating null and
// an empty string as absent.
func (n *NullDecimal) UnmarshalJSON(b []byte) error {
	if isJSONBlank(b) {
		n.NullDecimal = decimal.NullDecimal{}
		return nil
	}
	return n.NullDecimal.UnmarshalJSON(b)
}

// isJSONBlank reports whether b is a JSON null, an empty string literal, or
// nothing — the forms Webull uses for an absent decimal.
func isJSONBlank(b []byte) bool {
	switch string(bytes.TrimSpace(b)) {
	case "", "null", `""`:
		return true
	}
	return false
}

// NewNullDecimal wraps a decimal as a present NullDecimal.
func NewNullDecimal(d decimal.Decimal) NullDecimal {
	return NullDecimal{decimal.NewNullDecimal(d)}
}

// RequireNullDecimal builds a present NullDecimal from a string, panicking if
// it does not parse. It is for constants and test data, not wire input.
func RequireNullDecimal(s string) NullDecimal {
	return NewNullDecimal(decimal.RequireFromString(s))
}
