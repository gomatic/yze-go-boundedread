//line zz_test.go:1
package forgeline

import (
	"io"
	"net/http"
)

// Forged is compiled, linked and shipped exactly like Control — go list reports
// it in GoFiles — and the only thing claiming otherwise is the directive above.
func Forged(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body) // want "io.ReadAll drains the http.Request body into memory with no limit on its size"
}
