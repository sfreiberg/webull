package connect

import (
	"encoding/json"
	"testing"
)

// The token endpoint's response shape is not pinned by Webull's docs, so the
// decoder tolerates both quoted and bare numbers. Fuzzing pins that any
// response decodes to a Token or an error — never a panic — and that a
// successful decode round-trips through Marshal.
func FuzzTokenUnmarshalJSON(f *testing.F) {
	for _, seed := range []string{
		`{"access_token":"a","refresh_token":"r","token_type":"Bearer","expires_in":"1800","rt_expires_in":"1296000","created_at":"2026-09-02T12:00:00.000+0000","identity_id":"id"}`,
		`{"expires_in":1800,"rt_expires_in":1296000}`,
		`{"expires_in":null,"rt_expires_in":""}`,
		`{"expires_in":"soon"}`,
		`{"created_at":123}`,
		`{}`, `null`, `[]`, `"token"`, `{"expires_in":"9223372036854775808"}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var tok Token
		if err := json.Unmarshal(data, &tok); err == nil {
			if _, err := json.Marshal(tok); err != nil {
				t.Errorf("decoded token does not re-marshal: %v", err)
			}
		}
	})
}
