package forgeline

import (
	"io"
	"net/http"
)

// Helper sits at the matcher's LEFT edge — the underscore that separates a
// base name from the "_test.go" suffix. Its name ends in "test.go" and not in
// "_test.go", so the go tool compiles it into the package like any other
// source and the exemption must not reach it.
//
// This edge is not hypothetical and not latent: `find ~/src/github.com -name
// '*test.go' -not -name '*_test.go'` returns 39 files, among them
// net/http/httptest/httptest.go, gomatic/go-wofl/internal/pgtest/pgtest.go and
// gomatic/yze-go-errtest/errtest.go — this suite's own analyzer source. A
// matcher that dropped the underscore would exempt every one of them, and a
// denial-of-service rule would be switched off by naming a file httptest.go.
func Helper(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body) // want "io.ReadAll drains the http.Request body into memory with no limit on its size"
}
