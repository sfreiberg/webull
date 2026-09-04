package events

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the package if any test leaks a goroutine: every Subscribe
// opens a gRPC connection and a receive loop whose lifetime must end at
// Close, or a long-running caller accumulates both per reconnect.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
