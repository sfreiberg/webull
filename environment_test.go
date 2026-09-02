package webull

import (
	"testing"

	"github.com/sfreiberg/webull/internal/endpoints"
	"github.com/sfreiberg/webull/internal/transport"
)

// The public constant restates the transport's so godoc shows the value;
// this keeps the two from drifting.
func TestDefaultTimeoutMatchesTransport(t *testing.T) {
	if DefaultTimeout != transport.DefaultTimeout {
		t.Errorf("DefaultTimeout = %v, transport.DefaultTimeout = %v", DefaultTimeout, transport.DefaultTimeout)
	}
}

func TestEnvironmentValid(t *testing.T) {
	for env, want := range map[Environment]bool{
		Sandbox: true, Production: true, "": false, "staging": false,
	} {
		if got := env.Valid(); got != want {
			t.Errorf("%q.Valid() = %v, want %v", env, got, want)
		}
	}
}

func TestEnvironmentIsProduction(t *testing.T) {
	if !Production.IsProduction() {
		t.Error("Production.IsProduction() must be true")
	}
	for _, env := range []Environment{Sandbox, "", "staging"} {
		if env.IsProduction() {
			t.Errorf("%q must not report as production; safety guards depend on this", env)
		}
	}
}

func TestHostsAreDistinctPerEnvironment(t *testing.T) {
	for svc := range hosts[Production] {
		prod, sand := hosts[Production][svc], hosts[Sandbox][svc]
		if prod == "" || sand == "" {
			t.Errorf("service %q is missing a host in one environment", svc)
			continue
		}
		if prod == sand {
			t.Errorf("service %q uses the same host for production and sandbox (%q); "+
				"a test could reach production", svc, prod)
		}
	}
}

// The Connect API breaks the ".sandbox" naming pattern that the other services
// follow, so hosts must be looked up rather than derived. The connect package
// resolves these shared constants; this locks their values in.
func TestConnectHostsAreNotDerivable(t *testing.T) {
	if endpoints.ConnectProduction != "us-oauth-open-api.webull.com" {
		t.Errorf("connect production host = %q", endpoints.ConnectProduction)
	}
	if endpoints.ConnectSandbox != "oauth-open-api.sandbox.webull.com" {
		t.Errorf("connect sandbox host = %q", endpoints.ConnectSandbox)
	}
}

func TestConfigHostResolution(t *testing.T) {
	cfg := Config{Environment: Sandbox}
	got, err := cfg.host(serviceTrading)
	if err != nil {
		t.Fatal(err)
	}
	if got != "api.sandbox.webull.com" {
		t.Errorf("host = %q", got)
	}
}

func TestConfigHostOverride(t *testing.T) {
	cfg := Config{
		Environment:       Sandbox,
		EndpointOverrides: map[string]string{"trading": "127.0.0.1:8080"},
	}
	got, err := cfg.host(serviceTrading)
	if err != nil {
		t.Fatal(err)
	}
	if got != "127.0.0.1:8080" {
		t.Errorf("override ignored, got %q", got)
	}

	// An override for one service must not affect another.
	other, err := cfg.host(serviceStreaming)
	if err != nil {
		t.Fatal(err)
	}
	if other != "data-api.sandbox.webull.com" {
		t.Errorf("unrelated service was overridden: %q", other)
	}
}

func TestConfigHostEmptyOverrideIsIgnored(t *testing.T) {
	cfg := Config{Environment: Sandbox, EndpointOverrides: map[string]string{"trading": ""}}
	got, err := cfg.host(serviceTrading)
	if err != nil {
		t.Fatal(err)
	}
	if got != "api.sandbox.webull.com" {
		t.Errorf("an empty override should fall through to the default, got %q", got)
	}
}

func TestConfigHostUnknownEnvironment(t *testing.T) {
	cfg := Config{Environment: "staging"}
	if _, err := cfg.host(serviceTrading); err == nil {
		t.Fatal("expected an error for an unknown environment")
	}
}

func TestConfigHTTPClientDefault(t *testing.T) {
	var cfg Config
	if c := cfg.httpClient(); c == nil || c.Timeout != DefaultTimeout {
		t.Errorf("expected a default client with a %v timeout", DefaultTimeout)
	}
}

func TestEnvironmentString(t *testing.T) {
	if Sandbox.String() != "sandbox" || Production.String() != "production" {
		t.Error("unexpected Environment.String()")
	}
}

func TestConfigHostUnknownService(t *testing.T) {
	cfg := Config{Environment: Sandbox}
	if _, err := cfg.host(service("nonexistent")); err == nil {
		t.Fatal("expected an error for a service with no host entry")
	}
}
