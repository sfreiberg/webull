package marketdata

import (
	"context"

	"github.com/sfreiberg/webull/internal/query"
	"github.com/sfreiberg/webull/internal/transport"
)

// Client calls the Market Data API. It is safe for concurrent use.
type Client struct {
	doer *transport.Doer
	host string
}

// New returns a Client that sends requests through doer to host.
//
// It exists so the root webull package can compose a Client; callers of the
// SDK should use webull.NewClient.
func New(doer *transport.Doer, host string) *Client {
	return &Client{doer: doer, host: host}
}

func (c *Client) get(ctx context.Context, path string, q query.Params, out any) error {
	return classify(c.doer.Do(ctx, transport.Request{
		Method: "GET",
		Host:   c.host,
		Path:   path,
		Query:  q.Values(),
	}, out))
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return classify(c.doer.Do(ctx, transport.Request{
		Method: "POST",
		Host:   c.host,
		Path:   path,
		Body:   body,
	}, out))
}

// category returns c, or USStock when unset.
func category(c Category) Category {
	if c == "" {
		return USStock
	}
	return c
}
