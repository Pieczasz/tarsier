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
	if _, err := os.Stat(filepath.Join("..", "fixtures", "_badshop")); err != nil {
		t.Skip("fixtures not available")
	}
	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"check-cardinality", filepath.Join("..", "fixtures", "_badshop")})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "metrics/high-cardinality-label") && !strings.Contains(out, "high-cardinality") {
		// text format prints message, not always rule id - accept either
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

func TestCheckCardinalityCleanTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root := NewRootCommand()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"check-cardinality", dir})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ok: no cardinality findings") {
		t.Fatalf("want ok banner, got %q", buf.String())
	}
}

func TestCheckCardinalityFailOn(t *testing.T) {
	if _, err := os.Stat(filepath.Join("..", "fixtures", "_badshop")); err != nil {
		t.Skip("fixtures not available")
	}
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{
		"check-cardinality",
		"--fail-on", "warning",
		filepath.Join("..", "fixtures", "_badshop"),
	})
	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("want fail-on threshold error")
	}
	if !strings.Contains(err.Error(), "findings at or above") {
		t.Fatalf("got %v", err)
	}
}

func TestCheckCardinalityInvalidFailOn(t *testing.T) {
	dir := t.TempDir()
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"check-cardinality", "--fail-on", "loud", dir})
	err := root.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid --fail-on") {
		t.Fatalf("want invalid --fail-on, got %v", err)
	}
}
