package boundedread

import (
	"go/ast"
	"go/types"
	"testing"

	"github.com/stretchr/testify/assert"
)

// carrying is a scope with an empty carrier table, which is all the chain
// machinery needs.
func carrying() scope {
	return scope{rebound: map[rebinding]bool{}, carriers: map[selection]carrierID{}}
}

// TestAChainIsTheSamePathTwiceAndNothingElse pins interning, which is what lets
// a whole path stand as one comparable id: the same path met again yields the
// id it was first given, and a different path never yields that id. Without
// that, a cap and the read it governs would be two ids and the exemption would
// never apply; with it too loose, a cap would apply to a value it never touched.
func TestAChainIsTheSamePathTwiceAndNothingElse(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	known := carrying()
	a := types.NewVar(0, nil, "a", types.Typ[types.String])
	b := types.NewVar(0, nil, "b", types.Typ[types.String])

	first := known.rootOf(a)
	want.NotEqual(noCarrier, first, "a variable starts a chain")
	want.Equal(first, known.rootOf(a), "and starts the same one every time")
	want.NotEqual(first, known.rootOf(b), "another variable starts another")

	inner := known.extend(first, fieldIndex(0))
	want.NotEqual(first, inner, "a field reaches past the value it was selected from")
	want.Equal(inner, known.extend(first, fieldIndex(0)), "the same field, the same chain")
	want.NotEqual(inner, known.extend(first, fieldIndex(1)), "another field, another chain")
	want.NotEqual(inner, known.extend(known.rootOf(b), fieldIndex(0)),
		"the same field of another value is another chain")
}

// TestAnUnnamedValueNamesNothingAboveIt pins the poisoning rule. A value the
// rule cannot name — one built on the spot, or handed back by a call — has no
// chain, and a field selected out of it has none either. Minting one anyway
// would give `f().Request.Body` and `g().Request.Body` a single identity, which
// is the package-wide key this whole judgment exists to avoid.
func TestAnUnnamedValueNamesNothingAboveIt(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	known := carrying()

	want.Equal(noCarrier, known.rootOf(nil), "an identifier resolved to nothing starts no chain")
	want.Equal(noCarrier, known.extend(noCarrier, fieldIndex(0)), "and nothing extends it")
	want.Equal(noCarrier, known.extend(known.extend(noCarrier, fieldIndex(0)), fieldIndex(1)),
		"however many fields are selected out of it")
}

// TestAQualifiedIdentifierIsARootOfItsOwn pins the one selector the checker
// records no selection for: `pkg.Member` names a package's member rather than a
// value's field, so it is selected from no value and starts a chain instead of
// extending one.
func TestAQualifiedIdentifierIsARootOfItsOwn(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	known := carrying()
	member := ast.NewIdent("Member")
	declared := types.NewVar(0, nil, "Member", types.Typ[types.String])
	info := &types.Info{
		Uses:       map[*ast.Ident]types.Object{member: declared},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	qualified := &ast.SelectorExpr{X: ast.NewIdent("pkg"), Sel: member}

	want.Equal(known.rootOf(declared), known.walked(info, qualified, toTheValue),
		"a package member is the value itself, not a field of one")
	want.Equal(known.rootOf(declared), known.walked(info, qualified, throughHolder),
		"and it is that whichever reach asks")
}

// TestAssignedStreamFollowsOnlyWhatItCanName pins which assignment targets the
// rule records at all. A variable names itself. A target naming no value — an
// element of a slice, the field of a message built on the spot, an identifier
// the checker resolved to nothing — is recorded as nothing rather than as a
// claim about a stream the assignment never reached. Which STREAM each names is
// pinned by the fixtures, where the checker's own selections are available.
func TestAssignedStreamFollowsOnlyWhatItCanName(t *testing.T) {
	t.Parallel()
	want := assert.New(t)

	known := carrying()
	name := ast.NewIdent("req")
	declared := types.NewVar(0, nil, "req", types.Typ[types.String])
	info := &types.Info{
		Uses:       map[*ast.Ident]types.Object{name: declared},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}

	named, ok := known.assignedStream(info, name)
	want.True(ok)
	want.Equal(rebinding{object: declared}, named, "a variable names itself and needs no chain")

	_, ok = known.assignedStream(info, &ast.IndexExpr{X: name})
	want.False(ok, "an index names no stream")

	_, ok = known.assignedStream(info,
		&ast.SelectorExpr{X: &ast.CompositeLit{}, Sel: ast.NewIdent("Body")})
	want.False(ok, "a field of a value built on the spot names no stream")

	_, ok = known.assignedStream(info, ast.NewIdent("unresolved"))
	want.False(ok, "an identifier the checker resolved to nothing names no stream")
}
