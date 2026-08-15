package forgeline

import (
	"io"
	"net/http"
)

// Cased differs from a test file's name only in letter case. The go tool's own
// check is `strings.HasSuffix(name, "_test.go")` and is case-sensitive, so this
// is ordinary compiled source — verified on a case-INSENSITIVE darwin
// filesystem, where `go list` still reports it in GoFiles and never in
// TestGoFiles.
//
// Case is the third dimension this package's names would otherwise hold
// constant. Folding the name before matching is the ordinary instinct of anyone
// who has been bitten by a Windows or macOS path, and it exempts a file the
// build ships.
func Cased(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body) // want "io.ReadAll drains the http.Request body into memory with no limit on its size"
}
