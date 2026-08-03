package a

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"os"
)

// source holds a reader in a field rather than a parameter.
type source struct {
	r io.Reader
}

// envelope is a local type with a Body of its own: a field named Body that no
// peer fills is not an HTTP body.
type envelope struct {
	Body io.Reader
}

// Bounded is the conforming form: the read carries its own limit, so the
// argument is a call rather than a name and nothing is claimed about it.
func Bounded(r io.Reader) ([]byte, error) {
	return io.ReadAll(io.LimitReader(r, maxBytes))
}

// Local bounds the reader into a variable first. A local holds whatever was
// assigned to it — here, the very fix the rule asks for — so locals are silent.
func Local(r io.Reader) ([]byte, error) {
	limited := io.LimitReader(r, maxBytes)
	return io.ReadAll(limited)
}

// Rebound replaces the parameter itself with a bounded reader. What is read is
// no longer what arrived, so nothing is reported.
func Rebound(r io.Reader) ([]byte, error) {
	r = io.LimitReader(r, maxBytes)
	return io.ReadAll(r)
}

// Borrowed hands the parameter's address to a helper that may replace what it
// holds, so what is read afterwards is not knowably what arrived.
func Borrowed(r io.Reader) ([]byte, error) {
	bound(&r)
	return io.ReadAll(r)
}

// bound replaces a reader in place with a bounded one.
func bound(rp *io.Reader) {
	*rp = io.LimitReader(*rp, maxBytes)
}

// Constructed reads a reader built here from bytes already in hand.
func Constructed(raw []byte) ([]byte, error) {
	return io.ReadAll(bytes.NewReader(raw))
}

// Concrete reads a *bytes.Reader: a concrete type whose size the code can see.
func Concrete(r *bytes.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// Limited reads a reader that carries its own bound in its type: a concrete
// reader is one the code can see the shape of, and this one is bounded by
// construction.
func Limited(r *io.LimitedReader) ([]byte, error) {
	return io.ReadAll(r)
}

// Config reads a file the program itself opened. Reading an operator's config
// is not a denial of service, and the rule declines the judgment call.
func Config(f *os.File) ([]byte, error) {
	return io.ReadAll(f)
}

// Field reads a struct field rather than a parameter: what was stored there is
// not visible from the read, so nothing is claimed.
func (s *source) Field() ([]byte, error) {
	return io.ReadAll(s.r)
}

// Contents reads an unrelated Body field. Only an HTTP message body is a stream
// a peer controls.
func (e *envelope) Contents() ([]byte, error) {
	return io.ReadAll(e.Body)
}

// Stream copies to a response writer: it never accumulates in memory.
func Stream(w http.ResponseWriter, r io.Reader) (int64, error) {
	return io.Copy(w, r)
}

// Spill copies to a file: also not memory.
func Spill(f *os.File, r io.Reader) (int64, error) {
	return io.Copy(f, r)
}

// Counted is the conforming copy.
func Counted(r io.Reader) (string, error) {
	var buf bytes.Buffer
	_, err := io.CopyN(&buf, r, maxBytes)
	return buf.String(), err
}

// Spread passes a multi-valued call into io.Copy, so one argument expression
// fills both parameters and names no source of its own.
func Spread() (int64, error) {
	return io.Copy(pair())
}

// pair yields a destination and a source together.
func pair() (io.Writer, io.Reader) {
	var buf bytes.Buffer
	return &buf, bytes.NewReader(nil)
}

// Through writes a caller's reader onward through a buffered writer: the
// buffer is fixed and the bytes leave memory as they arrive.
func Through(w *bufio.Writer, r io.Reader) (int64, error) {
	return w.ReadFrom(r)
}

// Scan reads a caller's stream line by line. bufio.Scanner caps a token at 64
// KiB by default, so the default scanner is already bounded — reporting it
// would be a false positive, and the rule stays out.
func Scan(r io.Reader) int {
	lines := 0
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines++
	}
	return lines
}

// Describe calls a method whose receiver has no package — the universe error
// interface — and a builtin, neither of which is a drain.
func Describe(err error, raw []byte) int {
	_ = err.Error()
	return len(raw)
}

// Write is an io call that is not a drain at all.
func Write(w io.Writer, text string) (int, error) {
	return io.WriteString(w, text)
}

// Index assigns through an index expression, which names no variable the rule
// tracks, and negates a value, which is not an address.
func Index(raw []byte) []byte {
	raw[0] = ^raw[0]
	return raw
}
