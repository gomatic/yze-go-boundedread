// The scope: what a pass knows before it judges a call, and how far each fact
// reaches. A parameter is an object a value arrives on from outside; a
// rebinding is an assignment that may have replaced one, and it is evidence
// about the value it replaced, inside the function it was written in — and
// about nothing else.

package boundedread

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

// funcScope identifies the function a node was written in, by the position that
// function's declaration starts at. The scope is the OUTERMOST enclosing
// function, so a closure counts as part of the function that spells it: a
// literal is free to run before or after the read it sits beside — `apply :=
// func(){ req.Body = cap(req.Body) }; apply(); io.ReadAll(req.Body)` is
// ordinary capping code — and splitting the two apart would report it. This is
// safe only because the carrier is keyed alongside it: two handler closures in
// one function share this scope, and it is the carrier, not the scope, that
// keeps a cap on one request out of a read of the next. The zero value is the
// package block, where no statement is ordered against another.
type funcScope token.Pos

// rebinding is one stream an assignment may have replaced: the object the
// target names, the VALUE that object was selected from, and the function the
// assignment was written in.
//
// All three are needed, because neither the object nor the function alone
// identifies a stream. net/http declares a single Body field object that every
// request in the program selects, so an assignment keyed on the object alone
// speaks for every body in the package; and a closure written beside another
// closure shares its enclosing function, so `func(w, r){ r.Body = cap(r.Body) }`
// registered next to `func(w, r){ read(r.Body) }` would speak for a request it did
// not touch — which is what route registration looks like. The carrier is what
// distinguishes one request from the next.
type rebinding struct {
	object  types.Object
	carrier carrierID
	within  funcScope
}

// scope carries the pass-wide facts a decision rests on: which objects a value
// can arrive on from outside (parameters), which objects some assignment may
// have replaced and where (so nothing can be claimed about them there), and
// which source classes the run is claiming at all.
type scope struct {
	params   map[types.Object]bool
	rebound  map[rebinding]bool
	carriers map[selection]carrierID
	sources  sourceClass
}

// scopeOf collects the parameters and the assignment targets of the pass, under
// the selected source classes.
func scopeOf(info *types.Info, ins *inspector.Inspector) scope {
	known := scope{
		params:   map[types.Object]bool{},
		rebound:  map[rebinding]bool{},
		carriers: map[selection]carrierID{},
		sources:  selected,
	}
	nodes := []ast.Node{(*ast.FuncType)(nil), (*ast.AssignStmt)(nil), (*ast.UnaryExpr)(nil)}
	ins.WithStack(nodes, func(n ast.Node, isEntering bool, stack []ast.Node) bool {
		if isEntering {
			known.observe(info, n, declaredIn(stack))
		}
		return true
	})
	return known
}

// declaredIn is the function a node was written in — the outermost function on
// its syntactic stack — and the package block for a node written in none, which
// is where a type's signature and a package-level initialiser live.
func declaredIn(stack []ast.Node) funcScope {
	for _, node := range stack {
		switch node.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return funcScope(node.Pos())
		}
	}
	return funcScope(token.NoPos)
}

// observe folds one node into the scope: a signature contributes parameters,
// an assignment rebinds its targets within the function it was written in, and
// taking an address surrenders the operand — a pointer elsewhere may replace
// what it holds.
func (s scope) observe(info *types.Info, n ast.Node, within funcScope) {
	switch node := n.(type) {
	case *ast.FuncType:
		s.declareParams(info, node)
	case *ast.AssignStmt:
		s.rebind(info, node.Lhs, within)
	case *ast.UnaryExpr:
		s.rebind(info, addressed(info, node), within)
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

// rebind records the streams these expressions may replace, within the function
// the assignment was written in. A short variable declaration is included on
// purpose: `r, err := next()` in a function body assigns the parameter r, since
// parameters live in the body's own scope.
func (s scope) rebind(info *types.Info, targets []ast.Expr, within funcScope) {
	for _, target := range targets {
		if replaced, ok := s.assignedStream(info, target); ok {
			replaced.within = within
			s.rebound[replaced] = true
		}
	}
}

// assignedStream is the stream an assignment target names: a variable on its
// own, or a field together with the chain naming the value it was selected
// from. An expression naming neither — an element of a slice, the field of a
// message built on the spot — yields nothing, because an assignment this rule
// cannot tie to a value is not evidence about one. Recording it under the field
// alone is what let a single `_ = &(&http.Response{}).Body` speak for every
// body in sight.
func (s scope) assignedStream(info *types.Info, target ast.Expr) (rebinding, bool) {
	switch node := target.(type) {
	case *ast.Ident:
		if named := info.Uses[node]; named != nil {
			return rebinding{object: named}, true
		}
	case *ast.SelectorExpr:
		named := s.streamOf(info, node)
		if named.object != nil && named.carrier != noCarrier {
			return named, true
		}
	}
	return rebinding{}, false
}

// reading is the scope as one function sees it: the same pass-wide facts, plus
// which function's rebindings are in a position to govern the read being judged.
type reading struct {
	scope
	within funcScope
}

// in is the scope seen from inside the given function.
func (s scope) in(within funcScope) reading {
	return reading{scope: s, within: within}
}

// replaced reports a stream some assignment in this function may have replaced,
// which is the only place a rebinding is evidence about what a read receives.
func (r reading) replaced(stream rebinding) bool {
	stream.within = r.within
	return r.rebound[stream]
}

// checkCall reports a drain whose source is a stream of uncontrolled size.
func (r reading) checkCall(pass *analysis.Pass, call *ast.CallExpr) {
	drain, ok := drainOf(pass.TypesInfo, call)
	if !ok {
		return
	}
	if source, ok := r.uncontrolled(pass.TypesInfo, drain.source); ok {
		pass.Reportf(call.Pos(), message, drain.sink, source, drain.bounded)
	}
}
