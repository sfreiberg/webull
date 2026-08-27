package testutil

import (
	"testing"
	"time"
)

func TestIntegrationContextIsBoundedAndCancelled(t *testing.T) {
	var ctx = IntegrationContext(t)

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("context has no deadline")
	}
	if remaining := time.Until(deadline); remaining <= 0 || remaining > IntegrationTimeout {
		t.Errorf("deadline in %v, want within %v", remaining, IntegrationTimeout)
	}
	if ctx.Err() != nil {
		t.Errorf("context already done: %v", ctx.Err())
	}
}

func TestNewIntegrationClientSkipsWithoutCredentials(t *testing.T) {
	t.Setenv("WEBULL_APP_KEY", "")
	t.Setenv("WEBULL_APP_SECRET", "")

	reached := false
	ok := t.Run("inner", func(t *testing.T) {
		NewIntegrationClient(t)
		reached = true
	})
	if !ok {
		t.Fatal("the inner test failed; it should have skipped")
	}
	if reached {
		t.Fatal("NewIntegrationClient returned without credentials instead of skipping")
	}
}

func TestNewIntegrationClientWithCredentials(t *testing.T) {
	// NewClient performs no I/O, so placeholder credentials exercise the
	// whole construction path without touching the network.
	t.Setenv("WEBULL_APP_KEY", "placeholder-key")
	t.Setenv("WEBULL_APP_SECRET", "placeholder-secret")
	c := NewIntegrationClient(t)
	if c == nil || c.Environment().IsProduction() {
		t.Fatal("expected a sandbox client")
	}
}
