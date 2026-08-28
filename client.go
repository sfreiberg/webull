package webull

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sfreiberg/webull/internal/signing"
	"github.com/sfreiberg/webull/internal/transport"
	"github.com/sfreiberg/webull/marketdata"
	"github.com/sfreiberg/webull/trade"
)

// Client is the entry point to the Webull OpenAPI.
//
// Construct one with NewClient and share it: it is safe for concurrent use by
// multiple goroutines, and reusing it reuses the underlying HTTP connection
// pool. Constructing a Client performs no I/O.
//
// Long-lived streaming connections are deliberately not owned by Client. They
// are created explicitly so that their lifetime, and the goroutines they run,
// belong to the caller.
type Client struct {
	// Trade covers accounts, balances, positions, activities, instrument
	// reference data and orders.
	Trade *trade.Client
	// MarketData covers snapshots, quotes, ticks, bars and reference data.
	MarketData *marketdata.Client

	cfg  Config
	doer *transport.Doer
}

// NewClient returns a Client for the given configuration.
//
// It returns an error if credentials are missing or the environment is not one
// of Sandbox or Production. Errors never contain credential material.
func NewClient(cfg Config) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	userAgent := UserAgent()
	if cfg.UserAgent != "" {
		userAgent = cfg.UserAgent + " " + userAgent
	}

	// Clone the override map. Client documents itself as safe for concurrent
	// use, and sharing the caller's map would make that untrue the moment they
	// mutated it.
	if cfg.EndpointOverrides != nil {
		cfg.EndpointOverrides = maps.Clone(cfg.EndpointOverrides)
	}

	doer := &transport.Doer{
		HTTPClient:  cfg.httpClient(),
		Signer:      signing.New(cfg.AppKey, cfg.AppSecret),
		UserAgent:   userAgent,
		DecodeError: decodeAPIError,
		Retry:       transport.DefaultRetryPolicy(),
	}

	// Hosts are resolved once here. Overrides were cloned above, so the
	// resolution cannot change under a running client.
	tradingHost, err := cfg.host(serviceTrading)
	if err != nil {
		return nil, err
	}
	marketDataHost, err := cfg.host(serviceMarketData)
	if err != nil {
		return nil, err
	}

	return &Client{
		Trade:      trade.New(doer, tradingHost),
		MarketData: marketdata.New(doer, marketDataHost),
		cfg:        cfg,
		doer:       doer,
	}, nil
}

// Environment reports which deployment this client talks to.
func (c *Client) Environment() Environment { return c.cfg.Environment }

// get performs a signed GET against a service host and decodes the result.
func (c *Client) get(ctx context.Context, s service, path string, query url.Values, out any) error {
	host, err := c.cfg.host(s)
	if err != nil {
		return err
	}
	return c.doer.Do(ctx, transport.Request{
		Method: "GET",
		Host:   host,
		Path:   path,
		Query:  query,
	}, out)
}

// TokenCheckEnabled reports whether this deployment requires an access token in
// addition to the request signature.
//
// Webull gates token authentication per deployment rather than documenting it
// statically, so the answer must be asked for. It is not fetched during
// NewClient, because constructing a client should not perform I/O.
func (c *Client) TokenCheckEnabled(ctx context.Context) (bool, error) {
	var resp struct {
		TokenCheckEnabled bool `json:"token_check_enabled"`
	}
	if err := c.get(ctx, serviceTrading, "/openapi/config", nil, &resp); err != nil {
		return false, err
	}
	return resp.TokenCheckEnabled, nil
}

// errorPayload covers both of Webull's error shapes. Application errors carry
// error_code and message; errors from the API gateway carry only error_msg.
type errorPayload struct {
	Code      string `json:"error_code"`
	Message   string `json:"message"`
	GwMessage string `json:"error_msg"`
}

// decodeAPIError builds an *APIError from a failed response.
func decodeAPIError(resp transport.Response) error {
	err := &APIError{
		StatusCode: resp.StatusCode,
		RequestID:  requestIDOf(resp.Header),
		RetryAfter: retryAfterOf(resp.Header),
		Body:       resp.Body,
		Truncated:  resp.Truncated,
	}

	// Decode on a best-effort basis. encoding/json populates the fields it
	// understood before returning a type error, so a mismatch on one field
	// must not discard a message that decoded perfectly well.
	var payload errorPayload
	_ = json.Unmarshal(resp.Body, &payload)

	err.Code = payload.Code
	err.Message = payload.Message
	if err.Message == "" {
		err.Message = payload.GwMessage
	}
	return err
}

// requestIDOf extracts a correlation identifier if the response carries one.
func requestIDOf(h http.Header) string {
	for _, name := range []string{"X-Request-Id", "X-Request-ID", "Request-Id"} {
		if v := h.Get(name); v != "" {
			return v
		}
	}
	return ""
}

// retryAfterOf parses a Retry-After header, which may be either a number of
// seconds or an HTTP date. It returns zero when absent or unparseable.
func retryAfterOf(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}
