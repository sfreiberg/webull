package transport

import (
	"errors"
	"fmt"
	"testing"
)

type coded struct{ code string }

func (c *coded) Error() string     { return c.code }
func (c *coded) ErrorCode() string { return c.code }

func TestHasCode(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &coded{"A"})
	if !HasCode(err, "B", "A") {
		t.Error("should match a wrapped coded error")
	}
	if HasCode(err, "B") || HasCode(errors.New("plain"), "A") || HasCode(nil, "A") {
		t.Error("must not match other codes, uncoded errors or nil")
	}
}
