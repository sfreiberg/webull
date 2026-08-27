package webull

import (
	"context"
	"encoding/json"
	"net/url"

	"github.com/sfreiberg/webull/internal/signing"
	"github.com/sfreiberg/webull/internal/transport"
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

	return &Client{
		cfg: cfg,
		doer: &transport.Doer{
			HTTPClient:  cfg.httpClient(),
			Signer:      signing.New(cfg.AppKey, cfg.AppSecret),
			UserAgent:   userAgent,
			DecodeError: decodeAPIError,
			Retry:       transport.DefaultRetryPolicy(),
		},
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
func decodeAPIError(status int, requestID string, body []byte) error {
	err := &APIError{
		StatusCode: status,
		RequestID:  requestID,
		Body:       transport.TrimBody(body),
	}

	var payload errorPayload
	if jsonErr := json.Unmarshal(body, &payload); jsonErr == nil {
		err.Code = payload.Code
		err.Message = payload.Message
		if err.Message == "" {
			err.Message = payload.GwMessage
		}
	}
	return err
}
