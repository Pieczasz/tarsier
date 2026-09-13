package otelresource_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/Pieczasz/tarsier/internal/analyzers/otelresource"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), otelresource.Analyzer, "a")
}
