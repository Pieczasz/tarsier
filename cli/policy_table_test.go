package cli

import (
	"strings"
	"testing"

	"github.com/Pieczasz/tarsier/finding"
)

// Invariant after a successful validate: Block entries are trimmed known
// classes, and blockedRules expands every mapped rule for those classes.
func assertPolicyEnforceable(t *testing.T, p *Policy) {
	t.Helper()
	blocked := p.blockedRules()
	for _, class := range p.Block {
		if class != strings.TrimSpace(class) {
			t.Fatalf("Block retained padding %q", class)
		}
		rules, ok := blockableClasses[class]
		if !ok {
			t.Fatalf("Block has unknown class %q after validate", class)
		}
		for _, rule := range rules {
			if !blocked[rule] {
				t.Fatalf("class %q validated but rule %q missing from blockedRules", class, rule)
			}
		}
	}
}

func TestPolicyValidateTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		block   []string
		wantErr string
	}{
		{name: "plain class", block: []string{"unbounded-metric-labels"}},
		{name: "leading spaces", block: []string{"  unbounded-metric-labels"}},
		{name: "trailing spaces", block: []string{"missing-propagation  "}},
		{name: "both sides", block: []string{"\tsecrets-in-logs\n"}},
		{name: "two padded classes", block: []string{"  unbounded-metric-labels  ", " missing-propagation"}},
		{name: "empty entry", block: []string{"   "}, wantErr: "empty entry"},
		{name: "duplicate after trim", block: []string{"unbounded-metric-labels", "  unbounded-metric-labels  "}, wantErr: "duplicates"},
		{name: "layer 4", block: []string{" logs/unstructured-logging "}, wantErr: "layer 4"},
		{name: "unknown", block: []string{"  not-a-class  "}, wantErr: "not an opt-in"},
		{name: "empty block", block: nil, wantErr: "empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := &Policy{Version: 1, Block: append([]string(nil), tt.block...)}
			err := p.validate()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err=%v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertPolicyEnforceable(t, p)
		})
	}
}

func TestPolicyBlockingCountTable(t *testing.T) {
	t.Parallel()

	blocked := map[string]bool{"metrics/high-cardinality-label": true}
	tests := []struct {
		name string
		f    finding.Finding
		want int
	}{
		{name: "open high", f: finding.Finding{Rule: "metrics/high-cardinality-label", Confidence: "high"}, want: 1},
		{name: "medium skipped", f: finding.Finding{Rule: "metrics/high-cardinality-label", Confidence: "medium"}, want: 0},
		{name: "suppressed", f: finding.Finding{Rule: "metrics/high-cardinality-label", Confidence: "high", Status: finding.StatusSuppressed}, want: 0},
		{name: "other rule", f: finding.Finding{Rule: "otel/sdk-missing-resource-attrs", Confidence: "high"}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := policyBlockingCount([]finding.Finding{tt.f}, blocked)
			if got != tt.want {
				t.Fatalf("got %d want %d", got, tt.want)
			}
		})
	}
}
