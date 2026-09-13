package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCardinalityFiltersRules(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "..", "fixtures", "_badshop")); err != nil {
		t.Skip("fixtures not available")
	}
	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"check-cardinality", filepath.Join("..", "..", "fixtures", "_badshop")})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "metrics/high-cardinality-label") && !strings.Contains(out, "high-cardinality") {
		// text format prints message, not always rule id — accept either
		if !strings.Contains(out, "unbounded") && !strings.Contains(out, "billable") {
			t.Fatalf("expected cardinality output, got %q", out)
		}
	}
	if strings.Contains(out, "kafka-produce") || strings.Contains(out, "msgtrace") {
		t.Fatalf("non-cardinality finding leaked: %q", out)
	}
}

func TestIsCardinalityRule(t *testing.T) {
	if !isCardinalityRule("metrics/high-cardinality-label") || !isCardinalityRule("metrics/unbounded-label-value") {
		t.Fatal("expected match")
	}
	if isCardinalityRule("logs/unstructured-logging") {
		t.Fatal("unexpected match")
	}
}
