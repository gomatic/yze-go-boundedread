// The carrier: which VALUE a stream belongs to. net/http declares one Body
// field object for every request in the program, so a field names a kind of
// stream rather than a stream. What tells one request's body from the next is
// the chain of selections that names it — `req`, `a.Request`, `b.Request` — and
// a chain is interned here to a comparable id so a map key stays a fixed-size
// struct.

package boundedread

import (
	"go/ast"
	"go/types"
)

// carrierID identifies the value a field was selected from, as the CHAIN of
// selections that names it rather than as its last link. The last link is not
// enough for the same reason the field is not: `a.Request.Body` and
// `b.Request.Body` end in one Request field object, which every such struct in
// the program shares, so a cap on one would speak for the other. Chains are
// interned as they are met, so one comparable integer stands for a whole path.
type carrierID int

// noCarrier is what an expression naming no chain yields: a value built on the
// spot or returned by a call, which has none to give. An unknown root poisons
// every link above it, so a chain is either named all the way down or not named
// at all — otherwise `f().Request.Body` and `g().Request.Body` would share one
// identity, which is the package-wide key again in miniature.
const noCarrier carrierID = 0

// fieldIndex is a field's position in the struct that declares it, which is how
// the checker spells a selection: the fields it walked through, in order, to
// reach the one that was named. Working in those terms is what makes a promoted
// field and the same field spelled out in full arrive at one chain, since `w.Body`
// walks index 0 to reach the embedded request and `w.Request` names index 0.
type fieldIndex int

// reach says how much of a selection belongs to the chain: all of it when the
// selector names the VALUE a stream hangs off, and everything short of the last
// field when the selector names the stream itself, whose field is kept beside
// the chain rather than in it.
type reach int

// The two reaches a selection is walked to.
const (
	toTheValue reach = iota
	throughHolder
)

// selection is one link of a chain: either a root, which is the variable a
// chain starts at, or a step, which is a field's index within the value the
// chain has reached so far. A root carries its object and no index; a step
// carries an index and no object, so the two can share one table.
type selection struct {
	root types.Object
	from carrierID
	at   fieldIndex
}

// streamOf is the identity of the stream a selector names: the field it selects
// out, and the chain naming the value that field came from.
func (s scope) streamOf(info *types.Info, selector *ast.SelectorExpr) rebinding {
	return rebinding{carrier: s.walked(info, selector, throughHolder), object: info.Uses[selector.Sel]}
}

// walked is the chain a selector arrives at: the chain of the value it selects
// from, extended by the field indices the selection walks — all of them, or all
// but the last, depending on how far the caller means to reach.
//
// A selector the checker recorded no selection for is a qualified identifier
// naming a package's member, which is selected from no value at all and is a
// root of its own.
func (s scope) walked(info *types.Info, selector *ast.SelectorExpr, to reach) carrierID {
	found, ok := info.Selections[selector]
	if !ok {
		return s.rootOf(info.Uses[selector.Sel])
	}
	path := found.Index()
	if to == throughHolder {
		path = path[:len(path)-1]
	}
	chain := s.carrierOf(info, selector.X)
	for _, at := range path {
		chain = s.extend(chain, fieldIndex(at))
	}
	return chain
}

// carrierOf is the chain naming the value an expression denotes. Parentheses
// and a dereference change nothing about which value is meant — `(*rp).Body`
// and `rp.Body` are one stream — and a field of a field extends the chain
// rather than replacing it. Everything else names no value this rule can
// recognise on the reading side either, so it yields nothing rather than a
// wrong one.
func (s scope) carrierOf(info *types.Info, from ast.Expr) carrierID {
	switch node := from.(type) {
	case *ast.ParenExpr:
		return s.carrierOf(info, node.X)
	case *ast.StarExpr:
		return s.carrierOf(info, node.X)
	case *ast.Ident:
		return s.rootOf(info.Uses[node])
	case *ast.SelectorExpr:
		return s.walked(info, node, toTheValue)
	}
	return noCarrier
}

// rootOf is the chain a variable starts, and nothing for an identifier the
// checker resolved to no object.
func (s scope) rootOf(named types.Object) carrierID {
	if named == nil {
		return noCarrier
	}
	return s.mint(selection{root: named})
}

// extend is the chain one field longer, and nothing when the value being
// selected from is itself unnamed.
func (s scope) extend(from carrierID, at fieldIndex) carrierID {
	if from == noCarrier {
		return noCarrier
	}
	return s.mint(selection{from: from, at: at})
}

// mint is the identity of one link, given out the first time that link is met
// so the same path yields the same id on every later meeting.
func (s scope) mint(step selection) carrierID {
	if known, ok := s.carriers[step]; ok {
		return known
	}
	given := carrierID(len(s.carriers) + 1)
	s.carriers[step] = given
	return given
}
