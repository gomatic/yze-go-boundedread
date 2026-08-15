// Package forgeline pins that the test-file exemption is decided by the name
// the FileSet holds for a file, which no directive rewrites — not by the name a
// //line directive tells the position machinery to report.
//
// The three files here are the same unbounded read, and they differ only in
// what they claim to be. control.go claims nothing; forged.go is ordinary
// compiled source claiming a test name; forgeline_test.go is a real test file
// claiming a source name. The verdict follows what the go tool compiles, so
// only the test file is spared.
package forgeline

import (
	"io"
	"net/http"
)

// Control is the anchor: an unbounded read with no directive above it, so a
// silence anywhere else in this package is a silence the directive bought.
func Control(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body) // want "io.ReadAll drains the http.Request body into memory with no limit on its size"
}
