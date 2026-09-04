package wire

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Millis is a point in time transmitted as epoch milliseconds, which Webull
// sends sometimes as a JSON number and sometimes as a string. The zero value
// means the field was absent; a wire value of 0 is treated the same way,
// since nothing of interest happened at the epoch.
type Millis struct{ time.Time }

// UnmarshalJSON accepts a number, a numeric string, 0, or null. Floats are
// deliberately rejected: only integer and string milliseconds have been
// observed on the wire, and silently truncating a fraction would hide a
// shape change worth knowing about.
func (m *Millis) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		m.Time = time.Time{}
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return fmt.Errorf("webull: %q is not an epoch-millisecond time", s)
	}
	if n == 0 {
		m.Time = time.Time{}
		return nil
	}
	m.Time = time.UnixMilli(n).UTC()
	return nil
}

// MarshalJSON writes epoch milliseconds as a number, or null when zero.
func (m Millis) MarshalJSON() ([]byte, error) {
	if m.IsZero() {
		return []byte("null"), nil
	}
	return []byte(strconv.FormatInt(m.UnixMilli(), 10)), nil
}
