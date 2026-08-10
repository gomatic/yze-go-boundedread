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
		return bufferReadFrom(info, called, call)
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
		return copyDrain("io.Copy", info, call)
	case "io.CopyBuffer":
		return copyDrain("io.CopyBuffer", info, call)
	}
	return drainer{}, false
}

// copyDrain classifies an io.Copy or io.CopyBuffer that accumulates in memory —
// CopyBuffer's caller-supplied buffer changes how the bytes travel, not how
// many of them the destination keeps.
//
// The destination is judged before the source is read, which is also what
// makes reading it safe: a call may fill both parameters by spreading one
// multi-valued call — io.Copy(pair()) — and that single argument carries a
// tuple type, which is no destination at all.
func copyDrain(sink sinkName, info *types.Info, call *ast.CallExpr) (drainer, bool) {
	if !memorySink(info.TypeOf(call.Args[0])) {
		return drainer{}, false
	}
	return drainer{sink: sink, bounded: copyRemedy, source: call.Args[1]}, true
}

// bufferReadFrom classifies (*bytes.Buffer).ReadFrom, which is io.Copy into
// memory written as a method — in either spelling. The method-VALUE call
// `buf.ReadFrom(src)` carries the source as its only argument; the
// method-EXPRESSION call `(*bytes.Buffer).ReadFrom(&buf, src)` carries the
// receiver first and the source second.
func bufferReadFrom(info *types.Info, called *types.Func, call *ast.CallExpr) (drainer, bool) {
	if called.Name() != "ReadFrom" || !isNamed(called.Signature().Recv().Type(), "bytes", "Buffer") {
		return drainer{}, false
	}
	at := methodSourceIndex(info, call)
	if at >= len(call.Args) {
		// A method expression spread from one multi-valued call carries a
		// tuple no source claim can be made about.
		return drainer{}, false
	}
	return drainer{sink: "(*bytes.Buffer).ReadFrom", bounded: copyRemedy, source: call.Args[at]}, true
}

// methodSourceIndex is where a method call keeps its stream argument: after
// the receiver for a method expression, first otherwise.
func methodSourceIndex(info *types.Info, call *ast.CallExpr) int {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return 0
	}
	selection, ok := info.Selections[selector]
	if !ok {
		return 0
	}
	return sourceIndexByKind[selection.Kind()]
}

// sourceIndexByKind places the stream argument for every selection kind: a
// method expression carries the receiver first, so its source sits second.
var sourceIndexByKind = map[types.SelectionKind]int{
	types.MethodExpr: 1,
	types.MethodVal:  0,
	types.FieldVal:   0,
}

// memorySink reports a destination that accumulates without bound in memory.
func memorySink(destination types.Type) bool {
	return isNamed(destination, "bytes", "Buffer") || isNamed(destination, "strings", "Builder")
}
