// Package httponly holds one drain of each source class, under the DEFAULT
// selection (-sources=http).
//
// Only the HTTP bodies carry an expectation: the peer chooses those lengths and
// no caller can cap them, so the finding needs no judgment. The reader
// parameters beside them are unbounded by the letter and stay silent, because whether
// the bound belongs in the callee or in the caller is a design question this
// half of the rule declines to answer. Package `a` runs the same shapes under
// -sources=all, where they are all reported.
package httponly

import (
	"bytes"
	"io"
	"net"
	"net/http"
)

// Fetch reads a response body whole, which is reported under either selection.
func Fetch(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body) // want `io.ReadAll drains the http.Response body`
}

// Handle reads a request body whole, which is reported under either selection.
func Handle(req *http.Request) ([]byte, error) {
	return io.ReadAll(req.Body) // want `io.ReadAll drains the http.Request body`
}

// Absorb copies a request body into memory through the buffer's own method.
func Absorb(req *http.Request) (string, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(req.Body) // want `\(\*bytes.Buffer\).ReadFrom drains the http.Request body`
	return buf.String(), err
}

// Drain reads a caller's reader whole. Silent under the default selection: the
// caller may well be the one that should cap it.
func Drain(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// Close reads a caller's ReadCloser whole, and is silent for the same reason.
func Close(rc io.ReadCloser) ([]byte, error) {
	return io.ReadAll(rc)
}

// Serve reads a socket whole, and is silent for the same reason.
func Serve(conn net.Conn) ([]byte, error) {
	return io.ReadAll(conn)
}

// Buffer copies a caller's reader into memory, and is silent for the same reason.
func Buffer(r io.Reader) (string, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r)
	return buf.String(), err
}
