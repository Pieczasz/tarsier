package cli

import (
	"strings"
	"testing"

	"github.com/Pieczasz/tarsier/internal/finding"
)

func TestFailOnRank(t *testing.T) {
	t.Parallel()

	if _, err := failOnRank("loud"); err == nil || !strings.Contains(err.Error(), "invalid --fail-on") {
		t.Fatalf("typo'd threshold must fail, got %v", err)
	}
	n, err := failOnRank(failOnNone)
	if err != nil || n != 0 {
		t.Fatalf("none = %d, %v; want 0, nil", n, err)
	}
}

func TestBlockingCountIgnoresSuppressedAndBelowThreshold(t *testing.T) {
	t.Parallel()

	findings := []finding.Finding{
		{Severity: "warning", Status: finding.StatusOpen},
		{Severity: "warning", Status: finding.StatusSuppressed},
		{Severity: "info", Status: finding.StatusOpen},
		{Severity: "error", Status: finding.StatusOpen},
		{Severity: "warning", Confidence: "medium", Status: finding.StatusOpen},
	}
	warn, err := failOnRank(failOnWarning)
	if err != nil {
		t.Fatal(err)
	}
	if got := blockingCount(findings, warn); got != 2 {
		t.Fatalf("warning threshold: got %d, want 2 (open warning + error; medium skipped)", got)
	}
	none, err := failOnRank(failOnNone)
	if err != nil {
		t.Fatal(err)
	}
	if got := blockingCount(findings, none); got != 0 {
		t.Fatalf("none must never block, got %d", got)
	}
	errRank, err := failOnRank(failOnError)
	if err != nil {
		t.Fatal(err)
	}
	if got := blockingCount(findings, errRank); got != 1 {
		t.Fatalf("error threshold: got %d, want 1", got)
	}
}
