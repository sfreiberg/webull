package webull

import (
	"context"
	"net/url"
)

// ExportedGet exposes the internal GET path to tests in the webull_test
// package. It exists only in test builds.
func ExportedGet(ctx context.Context, c *Client, path string, out any) error {
	return c.get(ctx, serviceTrading, path, url.Values(nil), out)
}
