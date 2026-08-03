// Package b bounds the request body where it arrives, in the form the net/http
// package provides. The body read afterwards is the bounded one, so nothing is
// reported — including in Sibling, since the rule cannot tell which body a
// package-wide replacement covered and claims nothing rather than guess.
package b

import (
	"io"
	"net/http"
)

// maxBytes is the bound this package applies to every request body.
const maxBytes = 1 << 20

// Guarded caps the body before reading it: what is read is not what arrived.
func Guarded(w http.ResponseWriter, req *http.Request) ([]byte, error) {
	req.Body = http.MaxBytesReader(w, req.Body, maxBytes)
	return io.ReadAll(req.Body)
}

// Sibling reads a body this function did not cap itself.
func Sibling(req *http.Request) ([]byte, error) {
	return io.ReadAll(req.Body)
}
