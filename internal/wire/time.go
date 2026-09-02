// Package wire holds decoding helpers for value forms that recur across
// Webull's APIs, shared by the service packages.
package wire

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// isoLayout is Webull's ISO 8601 form: an offset without a colon, which
// time.RFC3339 rejects.
const isoLayout = "2006-01-02T15:04:05.000-0700"

// Time is a point in time transmitted in Webull's ISO 8601 form, such as
// "2026-08-27T04:00:00.000+0000". The zero value means the field was absent.
type Time struct{ time.Time }

// offsetWithoutColon matches a trailing "+0000"-style offset.
var offsetWithoutColon = regexp.MustCompile(`([+-]\d{2})(\d{2})$`)

// UnmarshalJSON accepts Webull's ISO form with any number of fractional
// digits, RFC 3339, a bare yyyy-MM-dd or yyyyMMdd date (as midnight UTC), or
// null.
func (t *Time) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		t.Time = time.Time{}
		return nil
	}
	if len(s) == len("2006-01-02") || len(s) == len("20060102") {
		layout := "2006-01-02"
		if len(s) == len("20060102") {
			layout = "20060102"
		}
		parsed, err := time.Parse(layout, s)
		if err != nil {
			return fmt.Errorf("webull: %q is not a recognised time", s)
		}
		t.Time = parsed
		return nil
	}
	// Normalise "+0000" to "+00:00" so the RFC 3339 parser, which accepts
	// any fractional precision, handles every variant.
	normalised := offsetWithoutColon.ReplaceAllString(s, "$1:$2")
	parsed, err := time.Parse(time.RFC3339Nano, normalised)
	if err != nil {
		return fmt.Errorf("webull: %q is not a recognised time", s)
	}
	t.Time = parsed.UTC()
	return nil
}

// MarshalJSON writes Webull's ISO form, or null when zero.
func (t Time) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.UTC().Format(isoLayout))
}
