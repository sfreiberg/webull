package trade

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/sfreiberg/webull/internal/transport"
)

// Client calls the Trading API. It is safe for concurrent use.
type Client struct {
	doer *transport.Doer
	host string
}

// New returns a Client that sends requests through doer to host.
//
// It exists so the root webull package can compose a Client; callers of the
// SDK should use webull.NewClient, which resolves the host for the configured
// environment and wires authentication.
func New(doer *transport.Doer, host string) *Client {
	return &Client{doer: doer, host: host}
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.doer.Do(ctx, transport.Request{
		Method: "GET",
		Host:   c.host,
		Path:   path,
		Query:  query,
	}, out)
}

// params accumulates query parameters, omitting empty values so that an unset
// optional field is not sent as an empty string.
type params url.Values

func (p params) set(key, value string) {
	if value != "" {
		url.Values(p).Set(key, value)
	}
}

// setList joins values with commas, which is how Webull accepts multi-value
// parameters such as symbol lists.
func (p params) setList(key string, values []string) {
	if len(values) > 0 {
		url.Values(p).Set(key, strings.Join(values, ","))
	}
}

func (p params) setInt(key string, value int) {
	if value != 0 {
		url.Values(p).Set(key, strconv.Itoa(value))
	}
}

func (p params) setBool(key string, value *bool) {
	if value != nil {
		if *value {
			url.Values(p).Set(key, "true")
		} else {
			url.Values(p).Set(key, "false")
		}
	}
}
