// Source judgment: which stream an expression reads, and whether its size is
// outside this code's reach.

package boundedread

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"
)

// uncontrolled names the stream an expression reads from when its size is
// outside this code's reach, and reports false for everything else.
//
// A call-expression argument is presumed to bound: the conforming forms —
// http.MaxBytesReader, io.LimitReader, a bound applied in a helper — all
// arrive as calls, and telling a bounding wrapper from a transparent one
// (bufio.NewReader adds no bound) needs knowledge of the callee this rule
// does not claim. That silence is a documented boundary, pinned by the
// wrapped fixture; so is a sink reached through a function VALUE
// (`f := io.ReadAll; f(body)`), which resolves to a variable, not a function.
func (r reading) uncontrolled(info *types.Info, source ast.Expr) (sourceName, bool) {
	switch node := unwrapped(info, source).(type) {
	case *ast.Ident:
		return r.callerStream(info, node)
	case *ast.SelectorExpr:
		return r.networkBody(info, node)
	}
	return "", false
}

// unwrapped strips the syntax that changes nothing about which stream is
// read: parentheses in every case, and a type assertion only when it asserts an
// INTERFACE — `resp.Body.(io.Reader)` is resp.Body, still opaque. An
// assertion to a concrete type is different in kind: it PROVES the dynamic
// type, which is exactly the "code can see what it is" evidence the concrete
// silence rests on, so `resp.Body.(*os.File)` stops the walk and stays silent.
func unwrapped(info *types.Info, source ast.Expr) ast.Expr {
	for {
		switch node := source.(type) {
		case *ast.ParenExpr:
			source = node.X
		case *ast.TypeAssertExpr:
			if !assertsInterface(info, node) {
				return source
			}
			source = node.X
		default:
			return source
		}
	}
}

// assertsInterface reports a type assertion whose asserted type is itself an
// interface, which reveals no concrete reader.
func assertsInterface(info *types.Info, assert *ast.TypeAssertExpr) bool {
	if assert.Type == nil {
		return false
	}
	at := info.TypeOf(assert.Type)
	if at == nil {
		return false
	}
	_, ok := at.Underlying().(*types.Interface)
	return ok
}

// callerStream names a stream the caller handed in: a parameter whose static
// type is a stream interface, still holding what it arrived with.
//
// This is the opt-in half of the rule. Such a read is unbounded by the letter,
// but the size may be the CALLER's to cap — so unless -sources=all selects the
// class, nothing here is claimed.
func (r reading) callerStream(info *types.Info, name *ast.Ident) (sourceName, bool) {
	if !r.sources.reportsCallerStreams() {
		return "", false
	}
	arrived := info.Uses[name]
	if !r.params[arrived] || r.replaced(rebinding{object: arrived}) {
		return "", false
	}
	stream, ok := streamInterface(arrived.Type())
	if !ok {
		return "", false
	}
	return sourceName("the " + string(stream) + " parameter " + strconv.Quote(name.Name)), true
}

// networkBody names a field of an HTTP message — a stream whose length the
// peer on the other end chooses — still holding what it arrived with.
//
// The judgment rests on the FIELD OBJECT the selector resolves to, not on the
// spelled receiver's type: a struct embedding *http.Request promotes the very
// same Body field, so `w.Body` and `w.Request.Body` are one stream and are
// judged alike.
//
// That object is one thing for the whole program, though — every request in
// every package selects the same Body — so it identifies the KIND of stream
// rather than the value. A rebinding is evidence about a value, so it is looked
// up under the value the body was selected from and the function the assignment
// was written in: a cap on one request is not a cap on the next one, and a cap
// in one function is not a cap on a body another function received.
func (r reading) networkBody(info *types.Info, selector *ast.SelectorExpr) (sourceName, bool) {
	stream := r.streamOf(info, selector)
	field, ok := stream.object.(*types.Var)
	if !ok || r.replaced(stream) {
		return "", false
	}
	carrier, ok := httpBodyCarrier(field)
	if !ok {
		return "", false
	}
	return sourceName("the " + string(carrier) + " " + strings.ToLower(selector.Sel.Name)), true
}

// httpBodyCarrier names the HTTP message a Body field belongs to, resolved by
// object identity against net/http's own declarations so promotion through an
// embedding struct cannot disguise the field.
func httpBodyCarrier(field *types.Var) (typeName, bool) {
	if !field.IsField() || field.Name() != "Body" || field.Pkg() == nil || field.Pkg().Path() != "net/http" {
		return "", false
	}
	for _, name := range []typeName{"Request", "Response"} {
		if declaresBody(field, name) {
			return "http." + name, true
		}
	}
	return "", false
}

// declaresBody reports whether the named net/http carrier's Body IS this field.
func declaresBody(field *types.Var, name typeName) bool {
	carrier := field.Pkg().Scope().Lookup(string(name))
	if carrier == nil {
		return false
	}
	found, _, _ := types.LookupFieldOrMethod(carrier.Type(), true, field.Pkg(), "Body")
	return found == field
}

// streamInterface names a stdlib stream interface — an interface declared in
// io or net carrying the Read method. An interface cannot advertise a length,
// so its size is unknowable from the call site; a concrete reader is left
// alone, since the code can see what it is. A generic parameter constrained to
// such an interface is that interface for every caller — `[R io.Reader]` is a
// caller-supplied reader spelled with a type parameter — so the constraint is
// judged in the parameter's place.
func streamInterface(carried types.Type) (typeName, bool) {
	if param, ok := types.Unalias(carried).(*types.TypeParam); ok {
		carried = param.Constraint()
	}
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
