package msgtrace_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/Pieczasz/tarsier/internal/analyzers/msgtrace"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), msgtrace.Analyzer, "a")
}
