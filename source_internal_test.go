package boundedread

import (
	"go/ast"
	"go/types"
	"path"
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

// namedTypeIn builds a named type declared in the given package. It is the one
// shape these cases need and the stdlib packages are stood in for rather than
// loaded, because what is under test is the decision this function makes about
// a type, not go/types' ability to find io on disk.
func namedTypeIn(at, name string, underlying types.Type) *types.Named {
	return types.NewNamed(types.NewTypeName(0, types.NewPackage(at, path.Base(at)), name, nil), underlying, nil)
}

// TestStreamInterfaceLeavesAConcreteReaderAloneAndJudgesAConstraintInItsPlace
// names the claims streamInterface makes about ITSELF, as against the sentence
// about interfaces carrying no length, which is a fact about Go that this
// function neither decides nor could falsify.
//
// Its first sentence is a conjunction of three conditions — declared in io or
// net, an interface, and carrying Read — and each is asserted here by a case
// that satisfies the other two. The Read condition was the one nothing pinned:
// hasRead has a unit test of its own, so deleting the CALL to it from readerIn
// left the whole suite green while every interface in io and net became a
// stream, which would report an io.Writer or a net.Addr parameter as an
// unbounded read. Found by an adversarial review of this test.
//
// The concrete half is the silence the whole rule rests on: a value whose type
// the code can SEE is left alone, so a struct declared in io and carrying a
// perfectly good Read method is not a stream interface — the discrimination is
// the underlying type, not the method. Getting that backwards would report
// every os.File in the fleet.
//
// The generic half is the claim that a constraint is judged in the parameter's
// place. It is asserted as an equality with the plain interface's answer rather
// than as "something was returned", because the claim is that the two are the
// SAME judgement: `[R io.Reader]` is a caller-supplied reader spelled with a
// type parameter. The negative direction is asserted too — a constraint
// declared outside io and net is refused through the parameter exactly as it
// would be directly, so unwrapping the parameter does not widen the rule.
//
// testdata/src/a/generic.go drives the generic case end to end, but end to end
// it is one `want` line on one shape; nothing pointed at the function that
// decides, and nothing at all covered the concrete-versus-interface split here.
func TestStreamInterfaceLeavesAConcreteReaderAloneAndJudgesAConstraintInItsPlace(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	reader := namedTypeIn("io", "Reader", ifaceOf(method("Read", 1, 2)).Complete())

	named, ok := streamInterface(reader)
	want.True(ok, "io.Reader is the stream interface every other case is measured against")
	want.Equal(typeName("io.Reader"), named)

	concrete := namedTypeIn("io", "File", types.NewStruct(nil, nil))
	_, ok = streamInterface(concrete)
	want.False(ok, "a concrete type declared in io is a reader the code can see, so it is left alone")

	writerOnly := namedTypeIn("io", "Writer", ifaceOf(method("Write", 1, 2)).Complete())
	_, ok = streamInterface(writerOnly)
	want.False(ok, "an interface declared in io that carries no Read drains nothing, so it is no stream")

	readerLike := namedTypeIn("io", "ByteReader", ifaceOf(method("ReadByte", 0, 2)).Complete())
	_, ok = streamInterface(readerLike)
	want.False(ok, "a method merely NAMED for reading is not Read, whichever package declares it")

	foreign := namedTypeIn("bufio", "Reader", ifaceOf(method("Read", 1, 2)).Complete())
	_, ok = streamInterface(foreign)
	want.False(ok, "a Read-carrying interface outside io and net is not one of the two named packages")

	constrained := types.NewTypeParam(types.NewTypeName(0, nil, "R", nil), reader)
	throughParam, ok := streamInterface(constrained)
	want.True(ok, "a parameter constrained to io.Reader is judged in the parameter's place")
	want.Equal(named, throughParam, "and is judged as the very same interface, not merely as some interface")

	constrainedForeign := types.NewTypeParam(types.NewTypeName(0, nil, "F", nil), foreign)
	_, ok = streamInterface(constrainedForeign)
	want.False(ok, "unwrapping the constraint judges it, it does not excuse it from the package test")
}
