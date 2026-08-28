package trade

import (
	"context"

	"github.com/sfreiberg/webull/internal/query"
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

func (c *Client) get(ctx context.Context, path string, q query.Params, out any) error {
	return c.doer.Get(ctx, c.host, path, q.Values(), out)
}

// post sends a JSON body. See transport.Doer.Post for why it never retries.
func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.doer.Post(ctx, c.host, path, body, out)
}

// transportResponse is transport.Response, named here so tests need not
// import the transport package.
type transportResponse = transport.Response
