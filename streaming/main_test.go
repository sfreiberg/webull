package streaming

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"go.uber.org/goleak"
)

// TestMain fails the package if any test leaks a goroutine: every Connect
// must be balanced by a Close that actually reaps the pump and the paho
// machinery, or a long-running caller accumulates goroutines per reconnect.
// The integration tests share the default transport, whose pooled idle
// HTTP/2 connections each hold a read-loop goroutine by design; those are
// drained before the check so only genuine leaks remain.
func TestMain(m *testing.M) {
	code := m.Run()
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
	if code == 0 {
		if err := goleak.Find(); err != nil {
			fmt.Fprintf(os.Stderr, "goleak: %v\n", err)
			code = 1
		}
	}
	os.Exit(code)
}
