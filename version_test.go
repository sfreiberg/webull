package webull

import (
	"runtime"
	"strings"
	"testing"
)

func TestVersionIsSet(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
	if strings.TrimSpace(Version) != Version {
		t.Errorf("Version has surrounding whitespace: %q", Version)
	}
}

func TestUserAgentIdentifiesSDKAndRuntime(t *testing.T) {
	got := UserAgent()

	if want := userAgentPrefix + "/" + Version; !strings.HasPrefix(got, want) {
		t.Errorf("UserAgent() = %q, want prefix %q", got, want)
	}
	for _, want := range []string{runtime.Version(), runtime.GOOS, runtime.GOARCH} {
		if !strings.Contains(got, want) {
			t.Errorf("UserAgent() = %q, missing %q", got, want)
		}
	}
}

func TestUserAgentIsSingleLine(t *testing.T) {
	// A User-Agent value containing a newline would allow header injection.
	if got := UserAgent(); strings.ContainsAny(got, "\r\n") {
		t.Errorf("UserAgent() = %q, must not contain CR or LF", got)
	}
}
