package webull_test

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestReadmeSnippetsMatchSource proves the README's Go blocks are the same
// text the compiler checked in readme_snippets_test.go. The snippet file
// indents each block one tab inside its wrapper and nests with tabs where
// the README nests with four spaces; normalising that away, the two must
// match byte for byte, in order.
func TestReadmeSnippetsMatchSource(t *testing.T) {
	readme, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	wants := regexp.MustCompile("(?ms)^```go\n(.*?)^```").FindAllStringSubmatch(string(readme), -1)

	snippets, err := os.ReadFile("readme_snippets_test.go")
	if err != nil {
		t.Fatal(err)
	}
	gots := regexp.MustCompile(`(?ms)^\t// readme:block\n(.*?)\n\t// readme:end`).FindAllStringSubmatch(string(snippets), -1)

	if len(wants) == 0 {
		t.Fatal("no Go blocks found in README.md")
	}
	if len(gots) != len(wants) {
		t.Fatalf("README.md has %d Go blocks, readme_snippets_test.go has %d", len(wants), len(gots))
	}

	for i := range wants {
		want := strings.TrimRight(wants[i][1], "\n")
		got := denest(gots[i][1])
		if got != want {
			t.Errorf("block %d differs.\nREADME:\n%s\nsnippet file:\n%s\nfirst difference:\n%s",
				i+1, want, got, firstDiff(want, got))
		}
	}
}

// denest strips the wrapper's one-tab indent and converts the remaining
// tab nesting to the README's four-space form.
func denest(region string) string {
	lines := strings.Split(region, "\n")
	for i, line := range lines {
		line = strings.TrimPrefix(line, "\t")
		rest := strings.TrimLeft(line, "\t")
		lines[i] = strings.Repeat("    ", len(line)-len(rest)) + rest
	}
	return strings.Join(lines, "\n")
}

func firstDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(w) && i < len(g); i++ {
		if w[i] != g[i] {
			return fmt.Sprintf("line %d:\nREADME:  %q\nsnippet: %q", i+1, w[i], g[i])
		}
	}
	return fmt.Sprintf("lengths differ: %d vs %d lines", len(w), len(g))
}
