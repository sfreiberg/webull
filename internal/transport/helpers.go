package transport

import (
	"context"
	"errors"
	"net/url"
)

// HasCode reports whether err carries one of the given Webull error codes.
// It is how service packages classify errors without importing the root
// package, which defines the error type.
func HasCode(err error, codes ...string) bool {
	var coded interface{ ErrorCode() string }
	if !errors.As(err, &coded) {
		return false
	}
	got := coded.ErrorCode()
	for _, c := range codes {
		if c == got {
			return true
		}
	}
	return false
}

// Get performs a signed GET against host and decodes the result into out.
func (d *Doer) Get(ctx context.Context, host, path string, query url.Values, out any) error {
	return d.Do(ctx, Request{Method: "GET", Host: host, Path: path, Query: query}, out)
}

// Post performs a signed POST with a JSON body. Nothing retries a POST: the
// outcome of a lost response is unknown, and in this API a replayed order is
// a duplicated order.
func (d *Doer) Post(ctx context.Context, host, path string, body, out any) error {
	return d.Do(ctx, Request{Method: "POST", Host: host, Path: path, Body: body}, out)
}
