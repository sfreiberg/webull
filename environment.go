package webull

import "fmt"

// Environment selects which Webull deployment the SDK talks to.
type Environment string

const (
	// Sandbox is Webull's test environment. Orders placed here are simulated.
	Sandbox Environment = "sandbox"

	// Production is the live environment. Orders placed here are real.
	Production Environment = "production"
)

// service identifies a part of Webull's API whose host is resolved on its
// own. Trading and market data currently share a host; the others differ.
type service string

const (
	serviceTrading    service = "trading"
	serviceMarketData service = "marketdata"
	serviceStreaming  service = "streaming"
	serviceEvents     service = "events"
)

// hosts maps each environment and service to its host.
//
// These are table entries rather than a derived pattern on purpose: deriving
// one host from another would silently produce a wrong name the day Webull
// breaks the pattern, as the Connect API hosts already do (see
// internal/endpoints, resolved by the connect package rather than here).
//
// Market data over HTTP is served by the trading host. The data-api hosts
// that Webull's SDKs list for market data do not answer HTTP requests; they
// are the MQTT streaming brokers, and are recorded here as serviceStreaming.
var hosts = map[Environment]map[service]string{
	Production: {
		serviceTrading:    "api.webull.com",
		serviceMarketData: "api.webull.com",
		serviceStreaming:  "data-api.webull.com",
		serviceEvents:     "events-api.webull.com",
	},
	Sandbox: {
		serviceTrading:    "api.sandbox.webull.com",
		serviceMarketData: "api.sandbox.webull.com",
		serviceStreaming:  "data-api.sandbox.webull.com",
		serviceEvents:     "events-api.sandbox.webull.com",
	},
}

// Valid reports whether e is a known environment.
func (e Environment) Valid() bool {
	_, ok := hosts[e]
	return ok
}

// String implements fmt.Stringer.
func (e Environment) String() string { return string(e) }

// host returns the host for a service, honouring any caller override.
func (c *Config) host(s service) (string, error) {
	if override, ok := c.EndpointOverrides[string(s)]; ok && override != "" {
		return override, nil
	}
	byService, ok := hosts[c.Environment]
	if !ok {
		return "", fmt.Errorf("webull: unknown environment %q", c.Environment)
	}
	h, ok := byService[s]
	if !ok {
		return "", fmt.Errorf("webull: no %s host for environment %q", s, c.Environment)
	}
	return h, nil
}

// IsProduction reports whether e is the live environment. Guard rails that must
// never fire against real accounts should test this rather than comparing
// strings, so that a future environment is not accidentally treated as safe.
func (e Environment) IsProduction() bool { return e == Production }
