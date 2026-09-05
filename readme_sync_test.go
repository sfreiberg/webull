package webull_test

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// blockRe matches the fenced Go blocks this test covers: ```go at column
// zero. looseFenceRe matches anything that looks like a Go fence anywhere —
// indented, or tagged golang — so a block the strict form misses fails the
// test instead of silently going uncompiled.
var (
	blockRe      = regexp.MustCompile("(?ms)^```go\n(.*?)^```")
	looseFenceRe = regexp.MustCompile("(?m)^[ \t]*```go(?:lang)?\\b")
	regionRe     = regexp.MustCompile(`(?ms)^\t// readme:block$\n(.*?)\n\t// readme:end$`)
	markerRe     = regexp.MustCompile(`(?m)^[ \t]*// readme:(block|end)$`)
)

// TestReadmeSnippetsMatchSource proves the README's Go blocks are the same
// text the compiler checked in readme_snippets_test.go. The snippet file
// indents each block one tab inside its wrapper and nests with tabs where
// the README nests with four spaces; normalising that away, the two must
// match byte for byte, in order.
func TestReadmeSnippetsMatchSource(t *testing.T) {
	readme := readNormalized(t, "README.md")
	snippets := readNormalized(t, "readme_snippets_test.go")

	wants := blockRe.FindAllStringSubmatch(readme, -1)
	if len(wants) == 0 {
		t.Fatal("no Go blocks found in README.md")
	}
	// Every Go-looking fence must be one the strict pattern covered: an
	// indented or ```golang fence would compile nowhere and rot silently.
	if loose := len(looseFenceRe.FindAllString(readme, -1)); loose != len(wants) {
		t.Fatalf("README.md has %d Go-looking fences but only %d matched ```go at column zero; use ```go, unindented, so the block is compiled and checked", loose, len(wants))
	}

	gots := regionRe.FindAllStringSubmatch(snippets, -1)
	// Guard the marker grammar itself: exactly two markers per region, at
	// one tab, or the region regex is quietly skipping something.
	if markers := len(markerRe.FindAllString(snippets, -1)); markers != 2*len(gots) {
		t.Fatalf("readme_snippets_test.go has %d readme markers but %d parsed regions; markers must be exactly `// readme:block` and `// readme:end` at one tab of indent", markers, len(gots))
	}
	if len(gots) != len(wants) {
		t.Fatalf("README.md has %d Go blocks, readme_snippets_test.go has %d; blocks pair by order, so add or remove the wrapper at the matching position", len(wants), len(gots))
	}

	for i := range wants {
		want := strings.TrimRight(wants[i][1], "\n")
		region := gots[i][1]
		if rawStringRe.MatchString(region) {
			t.Fatalf("block %d contains a multi-line raw string literal, whose leading tabs the indent normalisation would corrupt; use an interpreted string in README snippets", i+1)
		}
		got := strings.TrimRight(denest(region), "\n")
		if got != want {
			// Fail fast: with positional pairing, one inserted block would
			// otherwise cascade into a misleading mismatch for every block
			// after it.
			t.Fatalf("block %d differs (if you inserted or reordered blocks, wrappers must stay in README order).\nfirst difference:\n%s",
				i+1, firstDiff(want, got))
		}
	}
}

// rawStringRe detects a backquoted literal spanning lines.
var rawStringRe = regexp.MustCompile("`[^`]*\n[^`]*`")

// readNormalized reads a file with Windows line endings normalised, so a
// checkout under core.autocrlf still matches the LF-based patterns.
func readNormalized(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
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
