//line nottest.go:1
package forgeline

import (
	"io"
	"net/http"
)

// spared is the other direction: a real test file that a directive tells the
// position machinery to call nottest.go. It is test code, so the exemption
// still holds and nothing is reported here.
func spared(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}
