// Which class of uncontrolled source the rule claims, and the flag that selects
// it.
//
// The rule covers two source classes that differ in KIND, not in degree, and
// the split exists so each can be adopted on its own:
//
//   - An HTTP message body is uncontrolled by proof. The peer on the other end
//     of the connection chooses that length, this program never does, and no
//     caller can bound it on this code's behalf. Judging one takes no judgment,
//     so this class is the default and is fit to gate at zero.
//   - A caller-supplied reader PARAMETER is uncontrolled by the letter of the
//     rule, and who should bound it is a design question. An exported function
//     taking io.Reader — go-kv's LoadFromUnmarshaler, a CLI reading os.Stdin —
//     is genuinely unbounded here, but the size may well be the CALLER's to
//     cap, and capping it inside would break a legitimate whole-stream read.
//     Every finding in this class needs a human to decide, so it is opt-in.
//
// Neither class is dropped: `-sources=all` reports both, which is the rule as
// originally measured.

package boundedread

import (
	errs "github.com/gomatic/go-error"
)

// errUnknownSourceClass rejects a -sources value naming no class. The flag
// package prefixes it with the offending value, so the message is the remedy
// alone.
const errUnknownSourceClass errs.Const = `want "http" (HTTP message bodies only) ` +
	`or "all" (adds caller-supplied reader parameters)`

// sourceClass names the uncontrolled-source classes the rule reports.
type sourceClass string

// The selectable source classes.
const (
	// httpSources reports only HTTP message bodies — the class whose findings
	// need no judgment.
	httpSources sourceClass = "http"
	// allSources adds caller-supplied reader parameters, whose findings are
	// true by the letter and belong to a reviewer rather than to a gate.
	allSources sourceClass = "all"
)

// String is the flag's current value, as the flag package prints it in -help
// and in an error.
func (s sourceClass) String() string {
	return string(s)
}

// Set adopts a named class and rejects every other spelling, so an unknown
// selection fails when the flag is parsed rather than silently analysing
// nothing once per package.
//
// The receiver is a pointer because flag.Value requires one here: assigning
// through it is how the flag package delivers a parsed value. String needs no
// pointer, and a *sourceClass carries it either way.
func (s *sourceClass) Set(value string) error {
	switch class := sourceClass(value); class {
	case httpSources, allSources:
		*s = class
		return nil
	}
	return errUnknownSourceClass
}

// selected holds -sources. It defaults to the HTTP class: that half is provable
// rather than judged, so the analyzer's out-of-the-box behaviour is the half a
// repository can gate on.
var selected = httpSources

// reportsCallerStreams reports whether the caller-supplied reader parameter
// class is selected.
func (s sourceClass) reportsCallerStreams() bool {
	return s == allSources
}
