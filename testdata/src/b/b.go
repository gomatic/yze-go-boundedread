// Package b bounds the request body where the bound can govern the read. A cap
// silences the function that applies it — what that function reads is not what
// arrived — and says nothing about a body read in another function, which the
// cap never ran before. The two functions share net/http's Body FIELD OBJECT,
// not a body, and the field object is one thing for the whole program.
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

// Sibling reads a body no cap in this function ever replaced. Guarded's cap
// runs on Guarded's request, and this request never passed through it.
func Sibling(req *http.Request) ([]byte, error) {
	return io.ReadAll(req.Body) // want `io.ReadAll drains the http.Request body`
}

// touch caps nothing: it is never called, and the value it assigns is the value
// it read. An assignment that acquires none of the property the exemption
// exists for buys no silence — not here, and not for the drains around it.
func touch(req *http.Request) {
	req.Body = req.Body
}

// Applied caps the body inside a closure and reads it in the function body. The
// closure is part of the function, so the cap is still ordered before the read
// by code the rule can see, and the read stays silent.
func Applied(w http.ResponseWriter, req *http.Request) ([]byte, error) {
	apply := func() { req.Body = http.MaxBytesReader(w, req.Body, maxBytes) }
	apply()
	return io.ReadAll(req.Body)
}

// Captured caps the body and hands the read onward in a closure: the read is
// written inside the function that capped the body, so the cap governs it.
func Captured(w http.ResponseWriter, req *http.Request) func() ([]byte, error) {
	req.Body = http.MaxBytesReader(w, req.Body, maxBytes)
	return func() ([]byte, error) { return io.ReadAll(req.Body) }
}

// Edge caps the body where the request arrives and hands the request to a
// helper. This is the known COST of scoping the exemption to its evidence: the
// helper is reported even though this path capped the body, because proving
// that the request the helper received is the one Edge capped takes
// interprocedural dataflow the rule does not do. Bounding at the read, or
// handing the helper the bounded body rather than the request, answers it.
func Edge(w http.ResponseWriter, req *http.Request) ([]byte, error) {
	req.Body = http.MaxBytesReader(w, req.Body, maxBytes)
	return helper(req)
}

// helper reads a body some caller may or may not have capped.
func helper(req *http.Request) ([]byte, error) {
	return io.ReadAll(req.Body) // want `io.ReadAll drains the http.Request body`
}
