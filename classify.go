// Classification of one call: what a drain is, and which stream it reads.
//
// The two questions are deliberately separate. A SINK is a syntactic fact
// about the callee — io.ReadAll is a drain whatever it is handed. A SOURCE is
// a claim about a value, and every claim here has to be provable from the
// static type alone, which is what keeps the rule quiet.

package boundedread

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/types/typeutil"
)

// drainer is a call that pulls an entire stream into memory.
type drainer struct {
	source  ast.Expr
	sink    sinkName
	bounded remedy
}

// drainOf classifies a call as a drain, yielding the stream it reads.
func drainOf(info *types.Info, call *ast.CallExpr) (drainer, bool) {
	called, ok := typeutil.Callee(info, call).(*types.Func)
	if !ok {
		return drainer{}, false
	}
	if called.Signature().Recv() != nil {
		return bufferReadFrom(called, call)
	}
	return packageDrain(info, called, call)
}

// packageDrain classifies a package-level drain: ReadAll under either of its
// import paths, and the io.Copy that ends in memory. A callee no package
// declares — a method of the universe error interface — has no import path to
// ask for, and asking would dereference nothing.
func packageDrain(info *types.Info, called *types.Func, call *ast.CallExpr) (drainer, bool) {
	if called.Pkg() == nil {
		return drainer{}, false
	}
	switch called.Pkg().Path() + "." + called.Name() {
	case "io.ReadAll", "io/ioutil.ReadAll":
		sink := sinkName(called.Pkg().Name() + ".ReadAll")
		return drainer{sink: sink, bounded: limitRemedy, source: call.Args[0]}, true
	case "io.Copy":
		return copyDrain(info, call)
	}
	return drainer{}, false
}

// copyDrain classifies an io.Copy that accumulates in memory.
//
// The destination is judged before the source is read, which is also what
// makes reading it safe: a call may fill both parameters by spreading one
// multi-valued call — io.Copy(pair()) — and that single argument carries a
// tuple type, which is no destination at all.
func copyDrain(info *types.Info, call *ast.CallExpr) (drainer, bool) {
	if !memorySink(info.TypeOf(call.Args[0])) {
		return drainer{}, false
	}
	return drainer{sink: "io.Copy", bounded: copyRemedy, source: call.Args[1]}, true
}

// bufferReadFrom classifies (*bytes.Buffer).ReadFrom, which is io.Copy into
// memory written as a method.
func bufferReadFrom(called *types.Func, call *ast.CallExpr) (drainer, bool) {
	if called.Name() != "ReadFrom" || !isNamed(called.Signature().Recv().Type(), "bytes", "Buffer") {
		return drainer{}, false
	}
	return drainer{sink: "(*bytes.Buffer).ReadFrom", bounded: copyRemedy, source: call.Args[0]}, true
}

// memorySink reports a destination that accumulates without bound in memory.
func memorySink(destination types.Type) bool {
	return isNamed(destination, "bytes", "Buffer") || isNamed(destination, "strings", "Builder")
}

// uncontrolled names the stream an expression reads from when its size is
// outside this code's reach, and reports false for everything else.
func (s scope) uncontrolled(info *types.Info, source ast.Expr) (sourceName, bool) {
	switch node := source.(type) {
	case *ast.Ident:
		return s.callerStream(info, node)
	case *ast.SelectorExpr:
		return s.networkBody(info, node)
	}
	return "", false
}

// callerStream names a stream the caller handed in: a parameter whose static
// type is a stream interface, still holding what it arrived with.
//
// This is the opt-in half of the rule. Such a read is unbounded by the letter,
// but the size may be the CALLER's to cap — so unless -sources=all selects the
// class, nothing here is claimed.
func (s scope) callerStream(info *types.Info, name *ast.Ident) (sourceName, bool) {
	if !s.sources.reportsCallerStreams() {
		return "", false
	}
	arrived := info.Uses[name]
	if !s.params[arrived] || s.rebound[arrived] {
		return "", false
	}
	stream, ok := streamInterface(arrived.Type())
	if !ok {
		return "", false
	}
	return sourceName("the " + string(stream) + " parameter " + strconv.Quote(name.Name)), true
}

// networkBody names a field of an HTTP message — a stream whose length the
// peer on the other end chooses — still holding what it arrived with. The
// field is named rather than assumed: net/http declares exactly one stream on
// each carrier, Body, and only a stream can reach here at all, since this
// expression is what a drain was handed.
func (s scope) networkBody(info *types.Info, selector *ast.SelectorExpr) (sourceName, bool) {
	if s.rebound[info.Uses[selector.Sel]] {
		return "", false
	}
	carrier, ok := httpMessage(info.TypeOf(selector.X))
	if !ok {
		return "", false
	}
	return sourceName("the " + string(carrier) + " " + strings.ToLower(selector.Sel.Name)), true
}

// httpMessage names an HTTP request or response — the two carriers whose Body
// a peer fills. A Body field on anything else is this program's own vocabulary.
func httpMessage(carrier types.Type) (typeName, bool) {
	for _, name := range []typeName{"Request", "Response"} {
		if isNamed(carrier, "net/http", name) {
			return "http." + name, true
		}
	}
	return "", false
}

// streamInterface names a stdlib stream interface — an interface declared in
// io or net carrying the Read method. An interface cannot advertise a length,
// so its size is unknowable from the call site; a concrete reader is left
// alone, since the code can see what it is.
func streamInterface(carried types.Type) (typeName, bool) {
	for _, at := range []pkgPath{"io", "net"} {
		if named, ok := readerIn(carried, at); ok {
			return named, true
		}
	}
	return "", false
}

// readerIn names the type when it is a Read-carrying interface declared in the
// given package.
func readerIn(carried types.Type, at pkgPath) (typeName, bool) {
	named, ok := namedIn(carried, at)
	if !ok {
		return "", false
	}
	declared, ok := named.Underlying().(*types.Interface)
	if !ok || !hasRead(declared) {
		return "", false
	}
	return typeName(named.Obj().Pkg().Name() + "." + named.Obj().Name()), true
}

// hasRead reports an interface carrying the io.Reader method.
func hasRead(declared *types.Interface) bool {
	for at := range declared.NumMethods() {
		if isRead(declared.Method(at)) {
			return true
		}
	}
	return false
}

// isRead reports the Read(p []byte) (int, error) method. The name alone would
// admit ReadByte and ReadFrom, which drain nothing on their own.
func isRead(method *types.Func) bool {
	signature := method.Signature()
	return method.Name() == "Read" && signature.Params().Len() == 1 && signature.Results().Len() == 2
}

// isNamed reports the named type, seen through a pointer or an alias.
func isNamed(carried types.Type, at pkgPath, name typeName) bool {
	named, ok := namedIn(carried, at)
	return ok && named.Obj().Name() == string(name)
}

// namedIn resolves a type — pointer or not, alias or not — to a named type
// declared in the given package.
func namedIn(carried types.Type, at pkgPath) (*types.Named, bool) {
	named, ok := types.Unalias(deref(carried)).(*types.Named)
	if !ok {
		return nil, false
	}
	declaring := named.Obj().Pkg()
	if declaring == nil || declaring.Path() != string(at) {
		return nil, false
	}
	return named, true
}

// deref is the element of a pointer type, and the type itself otherwise.
func deref(carried types.Type) types.Type {
	if pointer, ok := types.Unalias(carried).(*types.Pointer); ok {
		return pointer.Elem()
	}
	return carried
}
