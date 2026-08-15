package boundedread

import (
	"errors"
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
)

// method builds an interface method with the given name and arity.
func method(name string, params, results int) *types.Func {
	signature := types.NewSignatureType(nil, nil, nil, argsOf(params), argsOf(results), false)
	return types.NewFunc(0, nil, name, signature)
}

// argsOf builds a tuple of the given width.
func argsOf(width int) *types.Tuple {
	args := make([]*types.Var, width)
	for at := range args {
		args[at] = types.NewVar(0, nil, "", types.Typ[types.Int])
	}
	return types.NewTuple(args...)
}

// ifaceOf builds an interface carrying the given methods.
func ifaceOf(methods ...*types.Func) *types.Interface {
	return types.NewInterfaceType(methods, nil)
}

// TestSetRejectsAnUnknownClassWithItsSentinel names errUnknownSourceClass's
// contract. A selection naming no class is refused with the sentinel — so the
// flag package reports a misspelling rather than analysing nothing — and the
// refusal leaves the previous selection untouched. Both real classes are
// accepted, which is what keeps the opt-in half reachable.
func TestSetRejectsAnUnknownClassWithItsSentinel(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	class := httpSources
	want.True(errors.Is(class.Set("https"), errUnknownSourceClass), "a near-miss names no class")
	want.True(errors.Is(class.Set(""), errUnknownSourceClass), "nor does the empty selection")
	want.Equal(httpSources, class, "a rejected selection changes nothing")

	want.NoError(class.Set("all"))
	want.Equal(allSources, class, "the opt-in class is selectable")
	want.NoError(class.Set("http"))
	want.Equal(httpSources, class, "and so is the default")
}

// TestOnlyAllReportsCallerStreams pins the split itself at its narrowest point:
// the caller-supplied reader class is claimed under `all` and under nothing
// else, which is what lets the HTTP half gate while this half stays a probe.
func TestOnlyAllReportsCallerStreams(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.True(allSources.reportsCallerStreams(), "all claims the caller's reader")
	want.False(httpSources.reportsCallerStreams(), "http claims only the body")
}

// TestStringIsTheSelectionAsWritten pins what the flag package prints back for
// -help and for a rejected value.
func TestStringIsTheSelectionAsWritten(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "http", httpSources.String())
	assert.Equal(t, "all", allSources.String())
}

// TestHasReadWantsTheReaderMethod pins what makes an interface a stream: the
// Read method of io.Reader, by name AND shape. The name alone would admit
// ReadByte, ReadFrom, and ReadAt — none of which drains anything on its own —
// so an interface carrying only those is not a stream this rule reports.
func TestHasReadWantsTheReaderMethod(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	want.True(hasRead(ifaceOf(method("Read", 1, 2))), "Read(p []byte) (int, error) is the reader")
	want.True(hasRead(ifaceOf(method("Close", 0, 1), method("Read", 1, 2))), "any position counts")

	want.False(hasRead(ifaceOf()), "an empty interface reads nothing")
	want.False(hasRead(ifaceOf(method("Write", 1, 2))), "a writer is not a reader")
	want.False(hasRead(ifaceOf(method("ReadByte", 0, 2))), "ReadByte drains nothing")
	want.False(hasRead(ifaceOf(method("Read", 2, 2))), "the wrong parameter count is not Read")
	want.False(hasRead(ifaceOf(method("Read", 1, 1))), "the wrong result count is not Read")
}

// TestNamedInRejectsAPackagelessType pins the guard that keeps the resolver
// from dereferencing nothing: the universe `error` is a named type with NO
// declaring package, and asking such a type for its import path would panic.
func TestNamedInRejectsAPackagelessType(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	universal := types.Universe.Lookup("error").Type()
	_, ok := namedIn(universal, "io")
	want.False(ok, "a type with no package is in no package")

	_, ok = namedIn(types.Typ[types.Int], "io")
	want.False(ok, "a basic type is not a named type")
}

// TestPackageDrainRejectsAPackagelessCallee pins the guard in front of the one
// dereference that can fail: a method of the universe `error` interface is a
// function no package declares, and asking it for an import path to match
// against would dereference nothing.
func TestPackageDrainRejectsAPackagelessCallee(t *testing.T) {
	t.Parallel()

	universal, _ := types.Universe.Lookup("error").Type().Underlying().(*types.Interface)
	_, ok := packageDrain(&types.Info{}, universal.Method(0), &ast.CallExpr{})
	assert.False(t, ok, "a func with no package drains nothing")
}

// TestDerefUnwrapsOnlyAPointer pins the one unwrapping the resolver does: a
// pointer yields its element, and everything else is already the type to judge.
func TestDerefUnwrapsOnlyAPointer(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	element := types.Typ[types.Int]
	want.Equal(element, deref(types.NewPointer(element)))
	want.Equal(element, deref(element))
}

// TestAssignedStreamNamesTheValueNotOnlyTheField pins which assignment targets
// the rule follows and what it records for each. A variable names itself. A
// field names the VALUE it was selected from as well, because net/http declares
// one Body field object for the whole program and the field on its own would
// speak for every request in sight. A target naming no value — an element of a
// slice, the field of a message built on the spot, an identifier the checker
// resolved to nothing — is recorded as nothing rather than as a claim about a
// stream the assignment never reached.
func TestAssignedStreamNamesTheValueNotOnlyTheField(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	first, second := ast.NewIdent("a"), ast.NewIdent("b")
	inner, body := ast.NewIdent("Request"), ast.NewIdent("Body")
	a := types.NewVar(0, nil, "a", types.Typ[types.String])
	b := types.NewVar(0, nil, "b", types.Typ[types.String])
	request := types.NewField(0, nil, "Request", types.Typ[types.String], false)
	field := types.NewField(0, nil, "Body", types.Typ[types.String], false)
	info := &types.Info{Uses: map[*ast.Ident]types.Object{
		first: a, second: b, inner: request, body: field,
	}}
	known := scope{rebound: map[rebinding]bool{}, carriers: map[selection]carrierID{}}

	named, ok := known.assignedStream(info, first)
	want.True(ok)
	want.Equal(rebinding{object: a}, named, "a variable names itself and needs no chain")

	selected, ok := known.assignedStream(info, &ast.SelectorExpr{X: first, Sel: body})
	want.True(ok)
	want.Equal(field, selected.object, "a field target names the field")
	want.NotEqual(noCarrier, selected.carrier, "and the value it was selected from")

	again, _ := known.assignedStream(info, &ast.SelectorExpr{X: first, Sel: body})
	want.Equal(selected, again, "the same path names the same stream every time it is met")

	elsewhere, _ := known.assignedStream(info, &ast.SelectorExpr{X: second, Sel: body})
	want.NotEqual(selected.carrier, elsewhere.carrier, "another value is another stream")

	nested, _ := known.assignedStream(info,
		&ast.SelectorExpr{X: &ast.SelectorExpr{X: first, Sel: inner}, Sel: body})
	want.NotEqual(selected.carrier, nested.carrier, "a.Body and a.Request.Body are different chains")

	sibling, _ := known.assignedStream(info,
		&ast.SelectorExpr{X: &ast.SelectorExpr{X: second, Sel: inner}, Sel: body})
	want.NotEqual(nested.carrier, sibling.carrier,
		"a.Request.Body and b.Request.Body share a Request field and nothing else")

	_, ok = known.assignedStream(info, &ast.IndexExpr{X: first})
	want.False(ok, "an index names no stream")

	_, ok = known.assignedStream(info, &ast.SelectorExpr{X: &ast.CompositeLit{}, Sel: body})
	want.False(ok, "a field of a value built on the spot names no stream")

	_, ok = known.assignedStream(info,
		&ast.SelectorExpr{X: &ast.SelectorExpr{X: &ast.CompositeLit{}, Sel: inner}, Sel: body})
	want.False(ok, "nor does a chain hanging off one — an unknown root names nothing above it")

	_, ok = known.assignedStream(info, ast.NewIdent("unresolved"))
	want.False(ok, "an identifier the checker resolved to nothing names no stream")

	want.Equal(noCarrier, known.link(noCarrier, nil), "an unresolved field extends no chain")
}

// TestDeclaredInIsTheOutermostFunction pins the scope a rebinding is recorded
// under: the outermost function on the node's stack, so a closure is part of
// the function that spells it and a cap applied in one governs a read in the
// other; and the package block for a node written in no function at all.
func TestDeclaredInIsTheOutermostFunction(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	file := &ast.File{}
	declared := &ast.FuncDecl{Type: &ast.FuncType{Func: token.Pos(10)}}
	literal := &ast.FuncLit{Type: &ast.FuncType{Func: token.Pos(20)}}

	want.Equal(funcScope(token.NoPos), declaredIn([]ast.Node{file}),
		"a node in no function belongs to the package block")
	want.Equal(funcScope(declared.Pos()), declaredIn([]ast.Node{file, declared, literal}),
		"a closure belongs to the function that spells it")
	want.Equal(funcScope(literal.Pos()), declaredIn([]ast.Node{file, literal}),
		"a literal with no declaration around it is its own scope")
}

// TestDeclareParamsToleratesASignatureWithoutParameters pins the guard against
// a parameter list that is absent rather than empty: nothing is declared, and
// nothing dereferences nil.
func TestDeclareParamsToleratesASignatureWithoutParameters(t *testing.T) {
	t.Parallel()

	known := scope{params: map[types.Object]bool{}, rebound: map[rebinding]bool{}}
	known.declareParams(&types.Info{}, &ast.FuncType{})
	assert.Empty(t, known.params)
}

// TestAddressedRecognisesAPointerByTypeNotOperator names the claim addressed
// documents: the address is recognised by the expression's TYPE, since only &x
// yields a pointer among the operators that can reach it — and the one other
// pointer-yielding unary, a receive from a channel of pointers, only ever adds
// silence, never a finding.
func TestAddressedRecognisesAPointerByTypeNotOperator(t *testing.T) {
	t.Parallel()

	operand := ast.NewIdent("x")
	pointer := types.NewPointer(types.Typ[types.Int])

	taken := &ast.UnaryExpr{Op: token.AND, X: operand}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{taken: {Type: pointer}}}
	assert.Equal(t, []ast.Expr{operand}, addressed(info, taken), "&x surrenders its operand")

	negated := &ast.UnaryExpr{Op: token.NOT, X: operand}
	info = &types.Info{Types: map[ast.Expr]types.TypeAndValue{negated: {Type: types.Typ[types.Bool]}}}
	assert.Nil(t, addressed(info, negated), "a non-pointer unary surrenders nothing")

	received := &ast.UnaryExpr{Op: token.ARROW, X: operand}
	info = &types.Info{Types: map[ast.Expr]types.TypeAndValue{received: {Type: pointer}}}
	assert.Equal(t, []ast.Expr{operand}, addressed(info, received),
		"a pointer received from a channel is treated as surrendered, which only ever adds silence")
}

// TestHTTPBodyCarrierDemandsNetHTTPsOwnBodyField pins the identity rule: a
// field is a carrier's Body only when it IS the field net/http declares on
// Request or Response. A synthetic net/http package declaring neither carrier
// proves both refusals — the lookup that finds no carrier, and the carrier
// walk that ends with no owner.
func TestHTTPBodyCarrierDemandsNetHTTPsOwnBodyField(t *testing.T) {
	t.Parallel()

	bare := types.NewPackage("net/http", "http")
	orphan := types.NewField(0, bare, "Body", types.Typ[types.String], false)
	_, ok := httpBodyCarrier(orphan)
	assert.False(t, ok, "a Body no carrier declares belongs to neither message")

	elsewhere := types.NewField(0, types.NewPackage("example.com/x", "x"), "Body", types.Typ[types.String], false)
	_, ok = httpBodyCarrier(elsewhere)
	assert.False(t, ok, "a Body outside net/http is this program's own vocabulary")

	named := types.NewField(0, bare, "Header", types.Typ[types.String], false)
	_, ok = httpBodyCarrier(named)
	assert.False(t, ok, "only Body is a stream")
}

// TestMethodSourceIndexDefaultsToTheFirstArgument pins the index rule's
// guards: a callee that is no selector, and a selector the checker recorded no
// selection for, both keep the source in first position.
func TestMethodSourceIndexDefaultsToTheFirstArgument(t *testing.T) {
	t.Parallel()

	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
	bare := &ast.CallExpr{Fun: ast.NewIdent("readFrom")}
	assert.Zero(t, methodSourceIndex(info, bare), "a non-selector callee has no receiver argument")

	sel := &ast.SelectorExpr{X: ast.NewIdent("buf"), Sel: ast.NewIdent("ReadFrom")}
	selected := &ast.CallExpr{Fun: sel}
	assert.Zero(t, methodSourceIndex(info, selected), "an unrecorded selection keeps first position")
}

// TestAssertsInterfaceRefusesWhatTheCheckerNeverSaw pins the guards: an
// assertion with no asserted type (the switch form never reaches expressions,
// but the guard holds regardless) and one the type info cannot resolve both
// reveal nothing.
func TestAssertsInterfaceRefusesWhatTheCheckerNeverSaw(t *testing.T) {
	t.Parallel()

	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	assert.False(t, assertsInterface(info, &ast.TypeAssertExpr{X: ast.NewIdent("x")}),
		"no asserted type reveals nothing")
	assert.False(t, assertsInterface(info, &ast.TypeAssertExpr{X: ast.NewIdent("x"), Type: ast.NewIdent("mystery")}),
		"an unresolved asserted type reveals nothing")
}
