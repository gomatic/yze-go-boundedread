// Package a reads streams whose size it does not control, in every shape the
// rule reports.
package a

import (
	"bytes"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

// maxBytes is the bound the conforming forms in silent.go apply.
const maxBytes = 1 << 20

// Drain reads a caller's reader whole.
func Drain(r io.Reader) ([]byte, error) {
	return io.ReadAll(r) // want `io.ReadAll drains the io.Reader parameter "r" into memory with no limit`
}

// Close reads a caller's ReadCloser whole: any io interface carrying Read is a
// stream of unknowable length.
func Close(rc io.ReadCloser) ([]byte, error) {
	return io.ReadAll(rc) // want `io.ReadAll drains the io.ReadCloser parameter "rc"`
}

// Serve reads a socket whole.
func Serve(conn net.Conn) ([]byte, error) {
	return io.ReadAll(conn) // want `io.ReadAll drains the net.Conn parameter "conn"`
}

// Fetch reads a response body whole: the peer chooses that length.
func Fetch(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body) // want `io.ReadAll drains the http.Response body`
}

// Handle reads a request body whole.
func Handle(req *http.Request) ([]byte, error) {
	return io.ReadAll(req.Body) // want `io.ReadAll drains the http.Request body`
}

// Legacy reads through the deprecated alias, which is the same drain.
func Legacy(r io.Reader) ([]byte, error) {
	return ioutil.ReadAll(r) // want `ioutil.ReadAll drains the io.Reader parameter "r"`
}

// Buffer copies a caller's reader into memory: ReadAll spelled as a copy.
func Buffer(r io.Reader) (string, error) {
	var buf bytes.Buffer
	_, err := io.Copy(&buf, r) // want `io.Copy drains the io.Reader parameter "r" .* bound it with io.CopyN`
	return buf.String(), err
}

// Build copies a caller's reader into a string builder — also memory.
func Build(r io.Reader) (string, error) {
	builder := &strings.Builder{}
	_, err := io.Copy(builder, r) // want `io.Copy drains the io.Reader parameter "r"`
	return builder.String(), err
}

// Absorb copies a response body into memory through the buffer's own method.
func Absorb(resp *http.Response) (string, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(resp.Body) // want `\(\*bytes.Buffer\).ReadFrom drains the http.Response body`
	return buf.String(), err
}

// drain is unexported: an internal helper drains just as unboundedly as an
// entry point, so visibility is not part of the rule.
func drain(r io.Reader) ([]byte, error) {
	return io.ReadAll(r) // want `io.ReadAll drains the io.Reader parameter "r"`
}
