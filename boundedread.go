// Package boundedread provides a go/analysis analyzer for the resiliency rule
// that a read of a stream whose size this code does not control must carry an
// explicit bound: io.ReadAll over a caller-supplied reader or an HTTP body
// allocates whatever the peer sends, which is a denial of service the program
// asked for. The conforming forms are io.ReadAll(io.LimitReader(r, max)),
// io.CopyN, and http.MaxBytesReader.
//
// # The sinks
//
// Three shapes pull an entire stream into memory and are reported: io.ReadAll
// (and the ioutil alias), io.Copy into a *bytes.Buffer or *strings.Builder —
// a ReadAll spelled differently — and (*bytes.Buffer).ReadFrom, which is that
// same copy written as a method. Copying to a file, a socket, or a
// ResponseWriter streams and allocates nothing unbounded, so it is not a sink.
//
// # The sources, and why the set is small
//
// A finding needs a source whose size is provably outside this code's reach.
// Exactly two shapes qualify, and -sources selects which of them are claimed:
//
//   - An HTTP message BODY — the Body field of an http.Request or an
//     http.Response. The peer chooses that length, never this program, and no
//     caller can bound it on this code's behalf. This class needs no judgment
//     and is the DEFAULT (-sources=http).
//   - A PARAMETER whose static type is a stream interface declared in io or
//     net — io.Reader, io.ReadCloser, net.Conn, and so on. This is the same
//     "untrusted input" that yze/fuzzreq classifies, seen from the reading
//     side: the value arrives from a caller, its dynamic type is invisible
//     here, and no interface can advertise a length. Such a read is unbounded
//     by the letter, but whether the BOUND belongs here or at the caller is a
//     design question a reviewer answers, so the class is opt-in
//     (-sources=all). See sources.go.
//
// Everything else is deliberately SILENT, because the alternative is a rule
// that fires on ordinary correct code:
//
//   - A LOCAL variable is silent, whatever its type. `lr := io.LimitReader(r,
//     max); io.ReadAll(lr)` is the conforming form and holds an io.Reader; a
//     rule that judged the interface alone would report the very fix it asks
//     for. Judging a local means tracking what was assigned to it, which is
//     dataflow this analyzer does not do.
//   - A CONCRETE type is silent — *bytes.Reader, *strings.Reader, an
//     embed.FS entry, *io.LimitedReader, and *os.File alike. The first four
//     are bounded by construction; a file the program itself named is the
//     judgment call the rule declines to make, since reading a config file the
//     operator chose is not a denial of service. The known cost of that line
//     is a concrete socket — *net.TCPConn and its siblings are unbounded and
//     go unreported, because a socket is nearly always held as net.Conn.
//   - A REBOUND source is silent: if any assignment in the package replaces
//     the parameter or the Body field (`r = io.LimitReader(r, max)`,
//     `req.Body = http.MaxBytesReader(w, req.Body, max)`), the value read is
//     no longer the one that arrived, so nothing is claimed about it.
//   - TEST files are out of scope: a test reads the fixture it wrote itself.
//
// # bufio.Scanner is deliberately absent
//
// A scanner is commonly named as an unbounded read, and the premise does not
// hold: bufio.Scanner caps a token at bufio.MaxScanTokenSize (64 KiB) and
// returns bufio.ErrTooLong past it, so the DEFAULT scanner is already bounded
// and reporting it would be a false positive. Only an explicit Buffer(buf,
// huge) removes the cap — and deciding which cap is "huge", and tying a Buffer
// call back to the source its scanner was built from, is dataflow and judgment
// rather than a provable fact. The rule stays silent rather than guess.
package boundedread

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	goyze "github.com/gomatic/go-yze"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// message is the diagnostic for an unbounded read of an uncontrolled stream.
const message = "%s drains %s into memory with no limit on its size; bound it with %s"

// The bounded forms a finding points at.
const (
	limitRemedy remedy = "io.LimitReader (or http.MaxBytesReader)"
	copyRemedy  remedy = "io.CopyN (or io.LimitReader)"
)

// sinkName is the call that performs the read, as a reader would write it.
type sinkName string

// sourceName describes the stream being read.
type sourceName string

// remedy is the bounded form that fixes a finding.
type remedy string

// pkgPath is a package's import path.
type pkgPath string

// typeName is a type's identifier within its package.
type typeName string

// Analyzer reports unbounded reads of streams whose size is beyond the code's sight.
var Analyzer = newAnalyzer()

// newAnalyzer builds the analyzer and declares -sources, which selects how much
// of the rule is claimed. See sources.go for why the two classes are separable.
func newAnalyzer() *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name: "boundedread",
		Doc: "reports io.ReadAll, io.Copy into memory, and bytes.Buffer.ReadFrom over an " +
			"HTTP body — or, with -sources=all, a caller-supplied reader too — with no size bound",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      run,
	}
	a.Flags.Var(&selected, "sources",
		"uncontrolled sources to report: http (HTTP bodies only) or all (adds caller-supplied readers)")
	return a
}

// Registration declares this analyzer to the yze framework.
var Registration = goyze.Registration{
	Name:       "boundedread",
	Categories: []goyze.Category{"patterns"},
	URL:        "https://docs.gomatic.dev/yze/boundedread",
	Analyzer:   Analyzer,
}

// run reports every drain of an uncontrolled stream outside test files.
func run(pass *analysis.Pass) (any, error) {
	ins, _ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	known := scopeOf(pass.TypesInfo, ins)
	ins.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call, _ := n.(*ast.CallExpr)
		if !inTestFile(pass, call.Pos()) {
			known.checkCall(pass, call)
		}
	})
	return nil, nil
}

// inTestFile reports a position inside a _test.go file. Test code reads the
// fixtures it wrote itself, so it is out of scope.
func inTestFile(pass *analysis.Pass, pos token.Pos) bool {
	return strings.HasSuffix(pass.Fset.Position(pos).Filename, "_test.go")
}

// scope carries the pass-wide facts a decision rests on: which objects a value
// can arrive on from outside (parameters), which objects some assignment may
// have replaced (so nothing can be claimed about them), and which source
// classes the run is claiming at all.
type scope struct {
	params  map[types.Object]bool
	rebound map[types.Object]bool
	sources sourceClass
}

// scopeOf collects the parameters and the assignment targets of the pass, under
// the selected source classes.
func scopeOf(info *types.Info, ins *inspector.Inspector) scope {
	known := scope{params: map[types.Object]bool{}, rebound: map[types.Object]bool{}, sources: selected}
	nodes := []ast.Node{(*ast.FuncType)(nil), (*ast.AssignStmt)(nil), (*ast.UnaryExpr)(nil)}
	ins.Preorder(nodes, func(n ast.Node) { known.observe(info, n) })
	return known
}

// observe folds one node into the scope: a signature contributes parameters,
// an assignment rebinds its targets, and taking an address surrenders the
// operand — a pointer elsewhere may replace what it holds.
func (s scope) observe(info *types.Info, n ast.Node) {
	switch node := n.(type) {
	case *ast.FuncType:
		s.declareParams(info, node)
	case *ast.AssignStmt:
		s.rebind(info, node.Lhs)
	case *ast.UnaryExpr:
		s.rebind(info, addressed(info, node))
	}
}

// addressed is the operand whose address a unary expression takes, and nothing
// for every other unary operator: negating or complementing a value hands no
// one the power to replace it.
//
// The address is recognised by the expression's TYPE rather than its operator,
// since only &x yields a pointer. A receive from a channel of pointers is the
// one other unary expression that does, and reading it surrenders nothing —
// but treating it as surrendered only ever adds silence, never a finding.
func addressed(info *types.Info, unary *ast.UnaryExpr) []ast.Expr {
	if _, ok := info.TypeOf(unary).(*types.Pointer); !ok {
		return nil
	}
	return []ast.Expr{unary.X}
}

// declareParams records the objects a signature's parameters bind.
func (s scope) declareParams(info *types.Info, signature *ast.FuncType) {
	if signature.Params == nil {
		return
	}
	for _, field := range signature.Params.List {
		s.declareNames(info, field.Names)
	}
}

// declareNames records the objects the given declarations bind.
func (s scope) declareNames(info *types.Info, names []*ast.Ident) {
	for _, name := range names {
		if declared, ok := info.Defs[name].(*types.Var); ok {
			s.params[declared] = true
		}
	}
}

// rebind records the objects these expressions may replace. A short variable
// declaration is included on purpose: `r, err := next()` in a function body
// assigns the parameter r, since parameters live in the body's own scope.
func (s scope) rebind(info *types.Info, targets []ast.Expr) {
	for _, target := range targets {
		if replaced := info.Uses[assignedIdent(target)]; replaced != nil {
			s.rebound[replaced] = true
		}
	}
}

// assignedIdent is the identifier an assignment target names — the variable
// itself, or the field of a selector — and nil for anything else, which names
// no object this rule tracks.
func assignedIdent(target ast.Expr) *ast.Ident {
	switch node := target.(type) {
	case *ast.Ident:
		return node
	case *ast.SelectorExpr:
		return node.Sel
	}
	return nil
}

// checkCall reports a drain whose source is a stream of uncontrolled size.
func (s scope) checkCall(pass *analysis.Pass, call *ast.CallExpr) {
	drain, ok := drainOf(pass.TypesInfo, call)
	if !ok {
		return
	}
	if source, ok := s.uncontrolled(pass.TypesInfo, drain.source); ok {
		pass.Reportf(call.Pos(), message, drain.sink, source, drain.bounded)
	}
}
