package a

import (
	"io"
	"strings"
	"testing"
)

// readAll drains a caller's reader with no bound at all, inside a test file: a
// test reads the fixture it wrote itself, so it is out of scope and nothing is
// reported here.
func readAll(t *testing.T, r io.Reader) string {
	t.Helper()

	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(raw)
}

// TestDrain exercises the entry point through that unbounded helper.
func TestDrain(t *testing.T) {
	if got := readAll(t, strings.NewReader("x")); got != "x" {
		t.Fatalf("readAll = %q", got)
	}
}
