package forgeline

import (
	"io"
	"net/http"
)

// Kit sits at the matcher's boundary. Its name CONTAINS "_test" and does not
// END in "_test.go", so the go tool compiles it into the package like any other
// source and the exemption must not reach it. A matcher widened from a suffix
// to a substring — the shape an author picks a filename to satisfy — silences
// this and nothing else in the package would notice.
func Kit(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body) // want "io.ReadAll drains the http.Request body into memory with no limit on its size"
}
