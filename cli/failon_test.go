package cli

import (
	"strings"
	"testing"

	"github.com/Pieczasz/tarsier/finding"
)

func TestFailOnRank(t *testing.T) {
	t.Parallel()

	tests := []struct {
		level string
		want  int
		err   bool
	}{
		{level: "", want: 0},
		{level: failOnNone, want: 0},
		{level: "hint", want: 1},
		{level: failOnInfo, want: 2},
		{level: failOnWarning, want: 3},
		{level: failOnError, want: 4},
		{level: "loud", err: true},
	}
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			t.Parallel()
			n, err := failOnRank(tt.level)
			if tt.err {
				if err == nil || !strings.Contains(err.Error(), "invalid --fail-on") {
					t.Fatalf("want invalid --fail-on error, got %v", err)
				}
				return
			}
			if err != nil || n != tt.want {
				t.Fatalf("failOnRank(%q) = %d, %v; want %d, nil", tt.level, n, err, tt.want)
			}
		})
	}
}

func TestSeverityRank(t *testing.T) {
	t.Parallel()

	tests := []struct {
		sev  string
		want int
	}{
		{sev: "hint", want: 1},
		{sev: "info", want: 2},
		{sev: "warning", want: 3},
		{sev: "error", want: 4},
		{sev: "unknown", want: 0},
		{sev: "", want: 0},
	}
	for _, tt := range tests {
		if got := severityRank(tt.sev); got != tt.want {
			t.Errorf("severityRank(%q) = %d, want %d", tt.sev, got, tt.want)
		}
	}
}

func TestFailOnThresholdErrorMessage(t *testing.T) {
	t.Parallel()

	err := failOnThresholdError{threshold: "warning", count: 3}
	if got := err.Error(); got != "3 findings at or above warning" {
		t.Fatalf("Error() = %q", got)
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
