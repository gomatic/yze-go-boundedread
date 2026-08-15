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
	"bufio"
	"bytes"
	"io"
	"net"
	"net/http"
	"os"
)

// fetched is a drain written in no function at all. A package-level initialiser
// belongs to the package block, which holds no cap of its own, so the body it
// reads is the one that arrived and the finding stands.
var fetched, _ = io.ReadAll(latest().Body) // want `io.ReadAll drains the http.Response body`

// surrendered takes the address of a Body that belongs to a message built on
// the spot. It names no value the rule can recognise, so it is not evidence
// about one and buys the drain above no silence — the package block is one
// scope shared by every file, and a rebinding recorded there under the field
// alone would be the package-wide hole again, spelled differently.
var surrendered = &(&http.Response{}).Body

// latest yields the response the package reads at initialisation.
func latest() *http.Response { return &http.Response{} }

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

// Parenthesized reads the same body through parentheses, which change nothing
// about which stream is read.
func Parenthesized(resp *http.Response) ([]byte, error) {
	return io.ReadAll((resp.Body)) // want `io.ReadAll drains the http.Response body`
}

// Asserted reads the same body through a type assertion: the asserted value IS
// the body.
func Asserted(req *http.Request) ([]byte, error) {
	return io.ReadAll(req.Body.(io.Reader)) // want `io.ReadAll drains the http.Request body`
}

// Buffered copies a request body into memory through CopyBuffer: the
// caller-supplied buffer changes how the bytes travel, not how many the
// destination keeps.
func Buffered(req *http.Request) (string, error) {
	var buf bytes.Buffer
	_, err := io.CopyBuffer(&buf, req.Body, make([]byte, 4096)) // want `io.CopyBuffer drains the http.Request body`
	return buf.String(), err
}

// wrapped embeds the request, promoting its Body: the promoted field is the
// same stream, judged by the field object rather than the spelled receiver.
type wrapped struct{ *http.Request }

// Promoted reads the embedded request's body through the promotion.
func Promoted(w wrapped) ([]byte, error) {
	return io.ReadAll(w.Body) // want `io.ReadAll drains the http.Request body`
}

// Explicit reads the same field through the full path, which must be judged
// identically.
func Explicit(w wrapped) ([]byte, error) {
	return io.ReadAll(w.Request.Body) // want `io.ReadAll drains the http.Request body`
}

// Rewrapped pins a documented boundary: a call-expression argument is presumed
// to bound, so a transparent wrapper — bufio.NewReader adds no bound — is
// silent. Telling a bounding wrapper from a transparent one needs knowledge of
// the callee this rule does not claim; widening this is a deliberate future
// decision.
func Rewrapped(resp *http.Response) ([]byte, error) {
	return io.ReadAll(bufio.NewReader(resp.Body))
}

// ThroughValue pins the second documented boundary: a sink reached through a
// function VALUE resolves to a variable, not a function, and is silent.
func ThroughValue(resp *http.Response) ([]byte, error) {
	drain := io.ReadAll
	return drain(resp.Body)
}

// AssertedConcrete pins the concrete-assertion silence: asserting the body to
// a concrete type PROVES the dynamic type, which is the "code can see what it
// is" evidence the concrete silence rests on — unlike the interface assertion
// above, which reveals nothing and is judged through.
func AssertedConcrete(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body.(*os.File))
}

// MethodExpression spells the buffer drain through the type: the receiver
// arrives first and the body second, and the source judgment follows it.
func MethodExpression(resp *http.Response) error {
	var buf bytes.Buffer
	_, err := (*bytes.Buffer).ReadFrom(&buf, resp.Body) // want `ReadFrom drains the http.Response body`
	return err
}

// pairBufBody spreads a receiver and a source out of one call.
func pairBufBody(resp *http.Response) (*bytes.Buffer, io.Reader) {
	return &bytes.Buffer{}, resp.Body
}

// SpreadMethodExpression spreads the method expression's whole argument list
// from one multi-valued call: the single argument carries a tuple no source
// claim can be made about, and the guard stays silent rather than reading an
// argument that is not there.
func SpreadMethodExpression(resp *http.Response) error {
	_, err := (*bytes.Buffer).ReadFrom(pairBufBody(resp))
	return err
}
