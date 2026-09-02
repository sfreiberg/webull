// Package signing implements Webull's HMAC request signature.
//
// Webull authenticates each request with a signature over a canonical string
// built from the request path, a fixed set of signature headers, the query
// parameters and a digest of the body. The algorithm is deterministic: given
// the same inputs, including timestamp and nonce, it always produces the same
// signature. That property is what makes it testable, so Signer takes its clock
// and nonce source as dependencies rather than reading the wall clock directly.
package signing

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Signature header names. These are part of Webull's wire protocol.
const (
	HeaderAppKey    = "x-app-key"
	HeaderTimestamp = "x-timestamp"
	HeaderVersion   = "x-signature-version"
	HeaderAlgorithm = "x-signature-algorithm"
	HeaderNonce     = "x-signature-nonce"
	HeaderSignature = "x-signature"
)

const (
	// signatureVersion is the only version Webull's clients send.
	signatureVersion = "1.0"

	// algorithm is HMAC-SHA256. This is deliberately not configurable.
	// Webull's Python SDK overrides any caller-selected algorithm with this
	// one, and its Go CLI defaults to HMAC-SHA1 with an MD5 body digest — an
	// option this package does not offer, because an SDK that can be
	// configured into SHA-1 is a liability rather than a feature.
	algorithm = "HMAC-SHA256"

	// timestampLayout is ISO-8601 UTC at second precision.
	timestampLayout = "2006-01-02T15:04:05Z"

	// keySuffix is appended to the app secret to form the HMAC key.
	keySuffix = "&"
)

// Request is the subset of an HTTP request that the signature covers.
type Request struct {
	// Host is the request host, without scheme or port.
	Host string
	// Path is the request path, beginning with a slash.
	Path string
	// Query holds the query parameters. A nil or empty map is valid.
	Query url.Values
	// Body is the exact serialized request body, or nil if there is none.
	// The digest covers these bytes, so they must be the same bytes that are
	// transmitted. Marshalling a value twice risks producing a signature over
	// content that differs from what is actually sent.
	Body []byte
}

// NonceFunc returns a unique value for each request.
type NonceFunc func() string

// Signer produces Webull request signatures. It is safe for concurrent use
// provided Now and Nonce are.
type Signer struct {
	AppKey    string
	appSecret string

	// Now returns the current time. Defaults to time.Now.
	Now func() time.Time
	// Nonce returns a per-request unique value. Defaults to a random UUID.
	Nonce NonceFunc
}

// New returns a Signer for the given credentials.
func New(appKey, appSecret string) *Signer {
	return &Signer{
		AppKey:    appKey,
		appSecret: appSecret,
		Now:       time.Now,
		Nonce:     uuid,
	}
}

// signatureHeaders builds the five signature headers every signed request
// carries. Both Sign and SignStream start from this one set, so a new
// header or version bump cannot be applied to one and missed in the other.
func (s *Signer) signatureHeaders() map[string]string {
	return map[string]string{
		HeaderAppKey:    s.AppKey,
		HeaderTimestamp: s.now().UTC().Format(timestampLayout),
		HeaderVersion:   signatureVersion,
		HeaderAlgorithm: algorithm,
		HeaderNonce:     s.nonce(),
	}
}

// Sign returns the headers to attach to req, including the signature itself.
// The returned map is freshly allocated and owned by the caller.
func (s *Signer) Sign(req Request) map[string]string {
	headers := s.signatureHeaders()
	headers[HeaderSignature] = s.sign(canonical(req, headers))
	return headers
}

// SignStream returns the metadata to attach to a gRPC request whose exact
// serialized message bytes are body. The canonical string differs from the
// HTTP form, matching Webull's event-stream signer: no path, host or query
// parameters participate; the body digest is lowercase hex rather than
// uppercase; and — a quirk shared by both official composers when there is
// no URI, verified against the live server — the sorted key=value pairs
// are joined by "=" rather than "&", with only the body digest appended by
// an "&".
func (s *Signer) SignStream(body []byte) map[string]string {
	headers := s.signatureHeaders()

	// Lowercased keys determine the sort order, exactly as in canonical.
	params := make(map[string]string, len(headers))
	for k, v := range headers {
		params[strings.ToLower(k)] = v
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+params[k])
	}

	var sb strings.Builder
	sb.WriteString(strings.Join(pairs, "="))
	if len(body) > 0 {
		sum := sha256.Sum256(body)
		sb.WriteString("&")
		sb.WriteString(hex.EncodeToString(sum[:]))
	}

	headers[HeaderSignature] = s.sign(escape(sb.String()))
	return headers
}

// sign computes the base64 HMAC-SHA256 of s over the signing key.
func (s *Signer) sign(canonical string) string {
	mac := hmac.New(sha256.New, []byte(s.appSecret+keySuffix))
	mac.Write([]byte(canonical))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// canonical builds the string that the signature is computed over.
//
// The construction is: the path, then every signature header and query
// parameter as sorted key=value pairs, then the uppercase hex SHA-256 of the
// body if there is one, joined by ampersands and percent-encoded as a whole.
func canonical(req Request, headers map[string]string) string {
	// Signature headers and the host participate with lowercased keys. The
	// lowercasing is not cosmetic: it determines the sort order below.
	params := make(map[string]string, len(headers)+len(req.Query)+1)
	params["host"] = req.Host
	for k, v := range headers {
		params[strings.ToLower(k)] = v
	}

	// Query parameters join the same map. A parameter colliding with a
	// signature header appends rather than replaces, matching Webull's
	// implementations.
	for k, vs := range req.Query {
		for _, v := range vs {
			if existing, ok := params[k]; ok {
				params[k] = existing + "&" + v
			} else {
				params[k] = v
			}
		}
	}

	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+params[k])
	}

	// With no path the canonical string is just the joined pairs. Emitting a
	// leading ampersand would produce a signature the server rejects.
	var sb strings.Builder
	if req.Path != "" {
		sb.WriteString(req.Path)
		sb.WriteString("&")
	}
	sb.WriteString(strings.Join(pairs, "&"))

	if len(req.Body) > 0 {
		sum := sha256.Sum256(req.Body)
		sb.WriteString("&")
		sb.WriteString(strings.ToUpper(hex.EncodeToString(sum[:])))
	}

	return escape(sb.String())
}

// escape percent-encodes everything outside the unreserved set. Unlike
// url.QueryEscape it encodes "/" as %2F and a space as %20 rather than "+".
func escape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

func (s *Signer) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Signer) nonce() string {
	if s.Nonce != nil {
		return s.Nonce()
	}
	return uuid()
}

// uuid returns a random version 4 UUID.
func uuid() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on any supported platform; if it somehow
		// does, a predictable nonce is worse than a loud failure.
		panic("webull: cannot read random bytes for signature nonce: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
