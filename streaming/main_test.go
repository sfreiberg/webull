package streaming

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the package if any test leaks a goroutine: every Connect
// must be balanced by a Close that actually reaps the pump and the paho
// machinery, or a long-running caller accumulates goroutines per reconnect.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
