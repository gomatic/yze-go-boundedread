// A generic reader parameter is a caller-supplied reader spelled with a type
// parameter: the constraint is the stream interface for every caller, so
// -sources=all judges it as it judges a plain io.Reader parameter.
package a

import "io"

// DrainGeneric reads a caller's generic reader whole.
func DrainGeneric[R io.Reader](r R) ([]byte, error) {
	return io.ReadAll(r) // want `io.ReadAll drains the io.Reader parameter "r"`
}
