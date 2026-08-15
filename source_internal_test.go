package boundedread

import (
	"go/ast"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestUnwrappedStopsAtAConcreteAssertion names the claim unwrapped documents,
// which is a claim about this function and not about Go: parentheses are
// stripped in every case, an assertion to an INTERFACE is stripped because it
// reveals nothing, and an assertion to a CONCRETE type stops the walk — it
// PROVES the dynamic type, which is the "code can see what it is" evidence the
// concrete silence rests on.
//
// The two assertion directions are exercised end to end by httponly.go's
// Asserted (reported) and AssertedConcrete (silent), but nothing named the
// function that decides between them, so the difference was a property no test
// could be pointed at. Stopping is asserted as identity rather than as
// "something was returned": the whole point is that the assertion expression
// itself survives, so a walk that unwrapped it anyway would still return a
// non-nil node.
func TestUnwrappedStopsAtAConcreteAssertion(t *testing.T) {
	t.Parallel()

	body := ast.NewIdent("body")
	assert.Same(t, body, unwrapped(&types.Info{}, &ast.ParenExpr{X: body}),
		"parentheses change nothing about which stream is read")

	readerName, concreteName := ast.NewIdent("Reader"), ast.NewIdent("File")
	reader := types.NewInterfaceType([]*types.Func{method("Read", 1, 2)}, nil)
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{
		readerName:   {Type: reader},
		concreteName: {Type: types.Typ[types.String]},
	}}

	through := &ast.TypeAssertExpr{X: body, Type: readerName}
	assert.Same(t, body, unwrapped(info, through),
		"an interface assertion is the same opaque stream, so the walk goes through it")

	proven := &ast.TypeAssertExpr{X: body, Type: concreteName}
	assert.Same(t, ast.Expr(proven), unwrapped(info, proven),
		"a concrete assertion proves the dynamic type, so the walk stops at the assertion itself")

	assert.Same(t, body, unwrapped(info, &ast.ParenExpr{X: &ast.ParenExpr{X: through}}),
		"the two strippings compose, and neither is reached only through the other")
}
