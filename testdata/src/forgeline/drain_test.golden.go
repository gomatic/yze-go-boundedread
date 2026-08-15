package forgeline

import (
	"io"
	"net/http"
)

// Golden sits INSIDE the matcher's literal. Its name contains "_test.go" and
// does not end in it, so `go list` reports it in GoFiles and the exemption must
// not reach it. A matcher widened from a suffix to a substring silences this
// and nothing else in the package would notice.
//
// The fleet holds no file of this shape — the same find that returns 39 files
// for the left edge returns none for this one — and that absence is the reason
// the case is here rather than the reason to skip it: a widening nothing
// exercises is a widening nothing can fail on. The escape is real either way,
// since an author who wants the exemption picks the name.
//
// The sibling escape, a package DIRECTORY named "*_test.go", is declined: it
// kills the same one widening, costs a second package in this corpus, and the
// dimension it adds beyond this file is the path-versus-base-name one, which no
// analyzer in the four repos keys on.
func Golden(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body) // want "io.ReadAll drains the http.Request body into memory with no limit on its size"
}
