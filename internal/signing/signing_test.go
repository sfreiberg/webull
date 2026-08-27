package signing

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixedSigner returns a Signer with a frozen clock and nonce, so signatures are
// reproducible and can be asserted exactly.
func fixedSigner() *Signer {
	s := New("test-app-key", "test-app-secret")
	s.Now = func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) }
	s.Nonce = func() string { return "00000000-0000-4000-8000-000000000000" }
	return s
}

func TestSignIsDeterministic(t *testing.T) {
	req := Request{Host: "api.example.com", Path: "/trading/accounts/list"}

	first := fixedSigner().Sign(req)
	second := fixedSigner().Sign(req)

	if first[HeaderSignature] != second[HeaderSignature] {
		t.Fatalf("same inputs produced different signatures:\n %q\n %q",
			first[HeaderSignature], second[HeaderSignature])
	}
}

// TestSignGoldenVector pins the exact output for known inputs. If this changes,
// the wire format changed, and every request this SDK makes would be rejected.
func TestSignGoldenVector(t *testing.T) {
	got := fixedSigner().Sign(Request{
		Host:  "api.example.com",
		Path:  "/trading/accounts/list",
		Query: url.Values{"account_id": {"ABC123"}},
	})

	want := map[string]string{
		HeaderAppKey:    "test-app-key",
		HeaderTimestamp: "2026-03-01T12:00:00Z",
		HeaderVersion:   "1.0",
		HeaderAlgorithm: "HMAC-SHA256",
		HeaderNonce:     "00000000-0000-4000-8000-000000000000",
		HeaderSignature: "77MONTBOg3HH8pIm09S3njPLqEd9ljKgpCjEj9xUqhg=",
	}

	for k, w := range want {
		if got[k] != w {
			t.Errorf("header %s = %q, want %q", k, got[k], w)
		}
	}
}

func TestCanonicalSortsParametersAndIncludesHost(t *testing.T) {
	headers := map[string]string{
		HeaderAppKey:    "key",
		HeaderTimestamp: "2026-03-01T12:00:00Z",
	}
	got := canonical(Request{
		Host:  "api.example.com",
		Path:  "/p",
		Query: url.Values{"zebra": {"1"}, "alpha": {"2"}},
	}, headers)

	// Decode enough to assert ordering without re-implementing the escaper.
	decoded, err := url.QueryUnescape(got)
	if err != nil {
		t.Fatalf("canonical output is not valid percent-encoding: %v", err)
	}

	want := "/p&alpha=2&host=api.example.com&x-app-key=key&x-timestamp=2026-03-01T12:00:00Z&zebra=1"
	if decoded != want {
		t.Errorf("canonical =\n %q\nwant\n %q", decoded, want)
	}
}

func TestCanonicalAppendsBodyDigest(t *testing.T) {
	body := []byte(`{"a":1}`)
	withBody := canonical(Request{Host: "h", Path: "/p", Body: body}, nil)
	without := canonical(Request{Host: "h", Path: "/p"}, nil)

	if withBody == without {
		t.Fatal("body did not affect the canonical string")
	}

	sum := sha256.Sum256(body)
	wantDigest := strings.ToUpper(hex.EncodeToString(sum[:]))

	decoded, _ := url.QueryUnescape(withBody)
	if !strings.HasSuffix(decoded, "&"+wantDigest) {
		t.Errorf("canonical string does not end with the uppercase body digest:\n got %q\n want suffix %q", decoded, "&"+wantDigest)
	}
}

func TestCanonicalEmptyBodyAddsNoDigest(t *testing.T) {
	empty := canonical(Request{Host: "h", Path: "/p", Body: []byte{}}, nil)
	nilBody := canonical(Request{Host: "h", Path: "/p", Body: nil}, nil)

	if empty != nilBody {
		t.Error("an empty body should be treated the same as no body")
	}
}

func TestCanonicalCollidingQueryKeyAppends(t *testing.T) {
	// A query parameter named like a signature header must append to it rather
	// than replace it, or the signature silently drops a signed value.
	got := canonical(Request{
		Host:  "h",
		Path:  "/p",
		Query: url.Values{"x-app-key": {"from-query"}},
	}, map[string]string{HeaderAppKey: "from-header"})

	decoded, _ := url.QueryUnescape(got)
	if !strings.Contains(decoded, "x-app-key=from-header&from-query") {
		t.Errorf("colliding key was not appended: %q", decoded)
	}
}

func TestEscapeEncodesSlashAndSpace(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"/a b", "%2Fa%20b"},
		{"a/b", "a%2Fb"},
		{"~-._", "~-._"}, // unreserved, must survive untouched
		{"a+b", "a%2Bb"},
	} {
		if got := escape(tc.in); got != tc.want {
			t.Errorf("escape(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSignNeverLeaksSecret(t *testing.T) {
	const secret = "super-secret-value"
	s := New("key", secret)
	s.Now = func() time.Time { return time.Unix(0, 0) }
	s.Nonce = func() string { return "nonce" }

	for k, v := range s.Sign(Request{Host: "h", Path: "/p"}) {
		if strings.Contains(v, secret) {
			t.Errorf("header %s contains the app secret", k)
		}
	}
}

func TestZeroValueSignerUsesDefaults(t *testing.T) {
	// A Signer built without New must still work rather than panic on a nil
	// clock or nonce func.
	var s Signer
	s.AppKey = "k"
	got := s.Sign(Request{Host: "h", Path: "/p"})

	if got[HeaderSignature] == "" {
		t.Error("expected a signature")
	}
	if got[HeaderNonce] == "" {
		t.Error("expected a generated nonce")
	}
}

func TestNonceIsUniquePerCall(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		n := uuid()
		if seen[n] {
			t.Fatalf("uuid repeated after %d calls: %s", i, n)
		}
		seen[n] = true
	}
}

func TestSignerIsSafeForConcurrentUse(t *testing.T) {
	s := New("key", "secret")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Sign(Request{Host: "h", Path: "/p"})
		}()
	}
	wg.Wait()
}
