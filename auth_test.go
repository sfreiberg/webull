package webull

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateAccessTokenDecodes(t *testing.T) {
	var gotPath, gotMethod string
	var gotBody []byte
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotBody, _ = io.ReadAll(r.Body)
		_, _ = w.Write([]byte(`{"token":"a1b2c3","expires_at":1760000000000,"status":"PENDING"}`))
	})
	tok, err := c.CreateAccessToken(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != "POST" || gotPath != "/auth/tokens/create" {
		t.Errorf("request = %s %s", gotMethod, gotPath)
	}
	if len(gotBody) != 0 {
		t.Errorf("create must send no body, got %q", gotBody)
	}
	if tok.Token != "a1b2c3" || tok.Status != TokenPending {
		t.Errorf("token = %+v", tok)
	}
	if want := time.UnixMilli(1760000000000).UTC(); !tok.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", tok.ExpiresAt, want)
	}
}

func TestCheckAccessTokenSendsTokenAndDecodesQuotedExpiry(t *testing.T) {
	var gotBody []byte
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		// The expiry as a quoted string, the other wire shape.
		_, _ = w.Write([]byte(`{"token":"a1b2c3","expires_at":"1760000000000","status":"NORMAL"}`))
	})
	tok, err := c.CheckAccessToken(context.Background(), "a1b2c3")
	if err != nil {
		t.Fatal(err)
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(gotBody, &body); err != nil || body.Token != "a1b2c3" {
		t.Errorf("body = %q", gotBody)
	}
	if tok.Status != TokenNormal || tok.ExpiresAt.IsZero() {
		t.Errorf("token = %+v", tok)
	}
}

func TestAccessTokenAbsentExpiryShapes(t *testing.T) {
	for name, body := range map[string]string{
		"null":   `{"token":"t","expires_at":null,"status":"NORMAL"}`,
		"empty":  `{"token":"t","expires_at":"","status":"NORMAL"}`,
		"zero":   `{"token":"t","expires_at":0,"status":"NORMAL"}`,
		"absent": `{"token":"t","status":"NORMAL"}`,
	} {
		var tok AccessToken
		if err := json.Unmarshal([]byte(body), &tok); err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if !tok.ExpiresAt.IsZero() {
			t.Errorf("%s: ExpiresAt = %v, want zero", name, tok.ExpiresAt)
		}
	}
	var tok AccessToken
	if err := json.Unmarshal([]byte(`{"expires_at":"soon"}`), &tok); err == nil {
		t.Error("a non-numeric expiry must be an error")
	}
}

func TestConfigAccessTokenIsSentAsHeader(t *testing.T) {
	var got string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("x-access-token")
		_, _ = w.Write([]byte(`{"token_check_enabled":true}`))
	}))
	t.Cleanup(srv.Close)
	c, err := NewClient(Config{
		AppKey: "k", AppSecret: "s", Environment: Sandbox,
		HTTPClient:  srv.Client(),
		AccessToken: "the-access-token",
		EndpointOverrides: map[string]string{
			"trading": strings.TrimPrefix(srv.URL, "https://"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.TokenCheckEnabled(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got != "the-access-token" {
		t.Errorf("x-access-token = %q", got)
	}
}

func TestNoAccessTokenMeansNoHeader(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if _, present := r.Header["X-Access-Token"]; present {
			t.Error("x-access-token must be absent when not configured")
		}
		_, _ = w.Write([]byte(`{"token_check_enabled":false}`))
	})
	if _, err := c.TokenCheckEnabled(context.Background()); err != nil {
		t.Fatal(err)
	}
}
