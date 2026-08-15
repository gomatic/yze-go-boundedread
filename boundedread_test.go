package boundedread_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"

	boundedread "github.com/gomatic/yze-go-boundedread"
)

// selecting sets -sources for one test and restores the default afterwards, so
// the two selections cannot leak into one another through the analyzer's
// process-global flag set.
func selecting(t *testing.T, class string) {
	t.Helper()
	require.NoError(t, boundedread.Analyzer.Flags.Set("sources", class))
	t.Cleanup(func() { _ = boundedread.Analyzer.Flags.Set("sources", "http") })
}

// TestAnUncontrolledStreamNeedsABound pins the rule at its widest selection.
// Under -sources=all, package a drains a caller's reader, a socket, and both
// HTTP bodies through every sink — io.ReadAll, its ioutil alias, io.Copy into a
// buffer or a builder, and (*bytes.Buffer).ReadFrom — and each is reported; the
// same file set holds the forms that must stay silent: the bounded read, the
// bounded copy, a local, a rebound parameter, a concrete reader, a file the
// program opened, a struct field, a Body that belongs to no HTTP message, a
// copy that streams onward, and the read inside a test file.
func TestAnUncontrolledStreamNeedsABound(t *testing.T) {
	selecting(t, "all")
	analysistest.Run(t, analysistest.TestData(), boundedread.Analyzer, "a")
}

// TestTheDefaultSelectionClaimsOnlyHTTPBodies pins the split that lets the
// halves be adopted apart. Under -sources=http, package httponly's HTTP bodies
// are reported — the peer chooses those lengths, so no judgment is owed — while
// the reader parameters beside them, identical in shape to the ones package a
// reports, stay silent.
//
// The selection is set explicitly rather than left ambient: the flag is
// process-global, so a test that relied on the default still being in place
// would pass or fail on the order the runner chose, and this suite shuffles.
func TestTheDefaultSelectionClaimsOnlyHTTPBodies(t *testing.T) {
	selecting(t, "http")
	analysistest.Run(t, analysistest.TestData(), boundedread.Analyzer, "httponly")
}

// TestHTTPIsTheRegisteredDefault pins which half a repository gets when it
// adopts the analyzer without configuring it: the provable one. The default is
// read from the flag's registration, which no test's Set call can disturb, so
// this holds whatever order the suite runs in.
func TestHTTPIsTheRegisteredDefault(t *testing.T) {
	assert.Equal(t, "http", boundedread.Analyzer.Flags.Lookup("sources").DefValue,
		"the zero-judgment half is what an unconfigured repository gates on")
}

// TestTheCallerStreamClassIsReachable pins that the opt-in half is not dropped:
// selecting it reports the very reader parameters the default leaves alone. The
// fixture is package a, whose expectations name them.
func TestTheCallerStreamClassIsReachable(t *testing.T) {
	selecting(t, "all")
	analysistest.Run(t, analysistest.TestData(), boundedread.Analyzer, "a")
}

// TestACapSilencesOnlyTheStreamItReplaces pins the exemption at the scope of
// its reason: the value that was replaced, in the function that replaced it.
//
// Package b's Guarded caps the request body before reading it, so what it reads
// is not what arrived and no finding is owed — there, and through a closure
// that caps or reads the same request (Applied, Captured), and through a
// pointer to it (Indirect). Everything that does not replace THAT value is
// reported: Sibling's request, which no cap in its own function touched; Other's
// second request, capped nowhere though its neighbour was; and the `/upload`
// handler in Routes, which shares an enclosing function with a capping handler
// and nothing else — the shape that route registration takes, and the one a
// function-only scope silenced. touch is the forgery that used to disable the
// package: an inert self-assignment on a request no other function holds, which
// now buys silence for nothing but itself.
//
// Edge/helper and Capping/Handle pin the COST of that scope rather than hide
// it: a bound applied in one function does not reach the read in another, so
// both readers are reported although both paths bound the body. That is not an
// argument for widening the exemption — Routes is what widening costs.
func TestACapSilencesOnlyTheStreamItReplaces(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), boundedread.Analyzer, "b")
}

// TestSourcesRejectsAnUnknownClass pins that a misspelled selection fails when
// the flag is parsed. The alternative — accepting it and reporting nothing —
// would read as a clean run, which is the worst outcome a gate can produce.
func TestSourcesRejectsAnUnknownClass(t *testing.T) {
	assert.Error(t, boundedread.Analyzer.Flags.Set("sources", "https"),
		"a near-miss spelling is not a class")
	assert.Error(t, boundedread.Analyzer.Flags.Set("sources", ""),
		"the empty selection names no class")
	assert.Equal(t, "http", boundedread.Analyzer.Flags.Lookup("sources").Value.String(),
		"a rejected selection leaves the default in place")
}

// TestRegistrationIsWellFormed pins the yze wiring.
func TestRegistrationIsWellFormed(t *testing.T) {
	assert.NoError(t, boundedread.Registration.Validate())
	assert.Equal(t, "yze/boundedread", boundedread.Registration.RuleID())
	assert.Same(t, boundedread.Analyzer, boundedread.Registration.Analyzer)
}

// TestADirectiveDoesNotRenameAFile pins the test-file exemption to the name the
// FileSet holds, which the judged file cannot rewrite. A `//line` directive is a
// compiler feature for generated code: it changes what fset.Position reports and
// nothing else — the go tool still compiles the file, go list still names it in
// GoFiles, and an importer still links it. So an exemption resolved through
// Position is one comment line away from being switched off by the very file it
// is judging, and this analyzer's rule is a denial-of-service bound.
//
// Package forgeline holds the same unbounded read three times. Control carries
// no directive; Forged is ordinary compiled source claiming a _test.go name and
// is reported anyway; spared is a real test file claiming a source name and is
// spared anyway. Both directions are needed: reading the wrong name forges a
// silence in one and a false positive in the other.
func TestADirectiveDoesNotRenameAFile(t *testing.T) {
	selecting(t, "http")
	analysistest.Run(t, analysistest.TestData(), boundedread.Analyzer, "forgeline")
}
