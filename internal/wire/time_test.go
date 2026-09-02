package wire

import (
	"encoding/json"
	"testing"
	"time"
)

func TestTimeDecodesWebullISOAndRFC3339(t *testing.T) {
	var v struct {
		W Time `json:"w"`
		R Time `json:"r"`
		Z Time `json:"z"`
	}
	if err := json.Unmarshal([]byte(`{"w":"2026-08-27T04:00:00.000+0000","r":"2026-08-27T04:00:00Z","z":null}`), &v); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 8, 27, 4, 0, 0, 0, time.UTC)
	if !v.W.Equal(want) || !v.R.Equal(want) || !v.Z.IsZero() {
		t.Errorf("w=%v r=%v z=%v", v.W, v.R, v.Z)
	}
	if err := json.Unmarshal([]byte(`{"w":"yesterday"}`), &v); err == nil {
		t.Error("an unrecognised time must be rejected")
	}
	b, _ := json.Marshal(Time{want})
	if string(b) != `"2026-08-27T04:00:00.000+0000"` {
		t.Errorf("marshal = %s", b)
	}
}

func TestTimeAcceptsAnyFractionalPrecisionWithFlatOffset(t *testing.T) {
	for _, in := range []string{`"2026-08-27T04:00:00+0000"`, `"2026-08-27T04:00:00.1+0000"`, `"2026-08-27T04:00:00.123456+0000"`, `"2026-08-27T00:00:00.000-0400"`} {
		var v Time
		if err := json.Unmarshal([]byte(in), &v); err != nil {
			t.Errorf("%s: %v", in, err)
		}
	}
	var v Time
	if err := json.Unmarshal([]byte(`"2026-08-27T00:00:00.000-0400"`), &v); err != nil || v.Hour() != 4 {
		t.Errorf("offset not applied: %v %v", v, err)
	}
}

func TestTimeAcceptsBareDate(t *testing.T) {
	var v Time
	if err := json.Unmarshal([]byte(`"2026-09-19"`), &v); err != nil {
		t.Fatal(err)
	}
	if !v.Equal(time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("got %v", v)
	}
	if err := json.Unmarshal([]byte(`"20260919"`), &v); err != nil {
		t.Fatal(err)
	}
	if !v.Equal(time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("compact date: got %v", v)
	}
	for _, in := range []string{`"2026-13-45"`, `"20261345"`} {
		if err := json.Unmarshal([]byte(in), &v); err == nil {
			t.Errorf("%s: an invalid date must be rejected", in)
		}
	}
}

func TestTimeZeroMarshalsAsNull(t *testing.T) {
	b, err := json.Marshal(Time{})
	if err != nil || string(b) != "null" {
		t.Errorf("zero marshal = %s, %v", b, err)
	}
}
