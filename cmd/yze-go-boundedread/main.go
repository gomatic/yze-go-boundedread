// Command yze-go-boundedread runs the boundedread analyzer as a standalone go/analysis
// checker (text and -json output, and usable as a `go vet -vettool`).
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	boundedread "github.com/gomatic/yze-go-boundedread"
)

// run is the analysis entry point, indirected so the binary's wiring is testable
// without invoking the real driver (which loads packages and exits the process).
var run = singlechecker.Main

func main() { run(boundedread.Analyzer) }
