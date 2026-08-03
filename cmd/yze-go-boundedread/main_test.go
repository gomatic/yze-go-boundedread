package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis"

	boundedread "github.com/gomatic/yze-go-boundedread"
)

func TestMainRunsTheAnalyzer(t *testing.T) {
	original := run
	t.Cleanup(func() { run = original })

	var got *analysis.Analyzer
	run = func(a *analysis.Analyzer) { got = a }

	main()

	require.NotNil(t, got)
	assert.Same(t, boundedread.Analyzer, got)
}
