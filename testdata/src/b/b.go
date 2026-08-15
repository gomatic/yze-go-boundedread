// Package b bounds request bodies where the bound can govern the read. A cap
// silences the read of the value it replaced, in the function that replaced it
// — what that read receives is not what arrived. It silences nothing else: not
// another function's request, which the cap never ran before, and not another
// request in its own function, which the cap never touched. What those all
// share is net/http's Body FIELD OBJECT, one thing for the whole program, and a
// field object is a kind of stream rather than a stream.
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

// Routes is route registration, where a cap and a read that have nothing to do
// with one another share an enclosing function. Each handler receives its own
// request, so the cap in the first says nothing about the read in the second —
// and the second is reported. Keyed on the function alone, the whole shape went
// silent, and route registration is where net/http code spends its time.
func Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/small", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		_, _ = io.ReadAll(r.Body)
	})
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body) // want `io.ReadAll drains the http.Request body`
	})
	return mux
}

// Other caps one request and reads another in the same breath. The cap is real
// and the read is still unbounded: the two requests share net/http's Body field
// and nothing else.
func Other(w http.ResponseWriter, req, other *http.Request) ([]byte, error) {
	req.Body = http.MaxBytesReader(w, req.Body, maxBytes)
	return io.ReadAll(other.Body) // want `io.ReadAll drains the http.Request body`
}

// Indirect caps and reads one request through a pointer to it. Parentheses and
// a dereference change nothing about which value is meant, so the cap governs
// the read.
func Indirect(w http.ResponseWriter, rp **http.Request) ([]byte, error) {
	(*rp).Body = http.MaxBytesReader(w, (*rp).Body, maxBytes)
	return io.ReadAll((*rp).Body)
}

// shared is a request the package holds for the life of the process, which is
// the one carrier two functions can genuinely have in common.
var shared *http.Request

// Adopt caps the request the package holds and reads it.
func Adopt(w http.ResponseWriter) ([]byte, error) {
	shared.Body = http.MaxBytesReader(w, shared.Body, maxBytes)
	return io.ReadAll(shared.Body)
}

// Later reads that same request from another function. One variable, one chain
// — what the two functions do not share is an ordering, and without one the cap
// in Adopt is no evidence about what this read receives. This is the case the
// function the assignment was written in is there for: the carrier cannot tell
// these two apart, because they really are the same value.
func Later() ([]byte, error) {
	return io.ReadAll(shared.Body) // want `io.ReadAll drains the http.Request body`
}

// pair holds two requests at once.
type pair struct{ first, second *http.Request }

// Paired caps one of a pair's requests and reads the other. Both bodies hang
// off the same value, so the value alone does not tell them apart — the field
// walked to reach each one is part of what names it, and a chain that stopped
// at the value would silence a body nothing capped.
func Paired(w http.ResponseWriter, held pair) ([]byte, error) {
	held.first.Body = http.MaxBytesReader(w, held.first.Body, maxBytes)
	return io.ReadAll(held.second.Body) // want `io.ReadAll drains the http.Request body`
}

// Edge caps the body where the request arrives and hands the request to a
// helper. This is the known COST of scoping the exemption to its evidence: the
// helper is reported even though this path capped the body, because proving
// that the request the helper received is the one Edge capped takes
// interprocedural dataflow the rule does not do. Handing the helper the bounded
// body rather than the request answers it.
func Edge(w http.ResponseWriter, req *http.Request) ([]byte, error) {
	req.Body = http.MaxBytesReader(w, req.Body, maxBytes)
	return helper(req)
}

// helper reads a body some caller may or may not have capped.
func helper(req *http.Request) ([]byte, error) {
	return io.ReadAll(req.Body) // want `io.ReadAll drains the http.Request body`
}

// Capping is the http.MaxBytesReader middleware idiom, and Handle is a handler
// behind it. This is the same cost in the shape that has NO remedy but a second
// bound at the read: http.Handler admits nothing but the request, so the
// middleware cannot hand the handler a bounded body. It is pinned rather than
// hidden. What would answer it is following the request into next.ServeHTTP,
// which is interprocedural dataflow this rule does not do — not a wider scope:
// widening to the enclosing function or the package reaches Handle only by also
// silencing every unbounded handler registered beside a capped one, which is
// what Routes shows.
func Capping(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}

// Handle reads a request some middleware may have bounded, and the rule cannot
// see which.
func Handle(w http.ResponseWriter, r *http.Request) {
	_, _ = io.ReadAll(r.Body) // want `io.ReadAll drains the http.Request body`
	_ = w
}
