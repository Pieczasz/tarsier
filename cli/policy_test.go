package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pieczasz/tarsier/finding"
)

func TestLoadPolicyAcceptsOptInClasses(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "policy.yaml")
	body := "version: 1\nblock:\n  - unbounded-metric-labels\n  - missing-propagation\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	blocked := p.blockedRules()
	if !blocked["metrics/high-cardinality-label"] || !blocked["msgtrace/kafka-produce-no-inject"] {
		t.Fatalf("blocked map incomplete: %v", blocked)
	}
}

func TestLoadPolicyRejectsLayer4Rule(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "policy.yaml")
	body := "version: 1\nblock:\n  - logs/unstructured-logging\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPolicy(path)
	if err == nil {
		t.Fatal("layer-4 rule must be rejected at parse time")
	}
	if !strings.Contains(err.Error(), "layer 4") {
		t.Fatalf("want layer rejection, got %v", err)
	}
}

func TestLoadPolicyRejectsUnknownClass(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "policy.yaml")
	body := "version: 1\nblock:\n  - otel/sdk-missing-resource-attrs\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPolicy(path)
	if err == nil {
		t.Fatal("non-opt-in rule must be rejected")
	}
	if !strings.Contains(err.Error(), "not an opt-in blocking class") {
		t.Fatalf("want class rejection, got %v", err)
	}
}

func TestPolicyBlockingCountIgnoresAdvisory(t *testing.T) {
	t.Parallel()

	blocked := map[string]bool{"metrics/high-cardinality-label": true}
	findings := []finding.Finding{
		{Rule: "metrics/high-cardinality-label", Confidence: "high", Status: finding.StatusOpen},
		{Rule: "metrics/high-cardinality-label", Confidence: "medium", Status: finding.StatusOpen},
		{Rule: "metrics/high-cardinality-label", Confidence: "high", Status: finding.StatusSuppressed},
		{Rule: "otel/sdk-missing-resource-attrs", Confidence: "high", Status: finding.StatusOpen},
	}
	if got := policyBlockingCount(findings, blocked); got != 1 {
		t.Fatalf("policyBlockingCount = %d, want 1", got)
	}
}

func TestLoadPolicyTrimsBlockEntries(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "policy.yaml")
	body := "version: 1\nblock:\n  - \"  unbounded-metric-labels  \"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := LoadPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	if !p.blockedRules()["metrics/high-cardinality-label"] {
		t.Fatalf("padded block entry must still expand; Block=%v blocked=%v", p.Block, p.blockedRules())
	}
}

func TestCheckCommandExitsOnBlockedClass(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	policy := filepath.Join(dir, "policy.yaml")
	if err := os.WriteFile(policy, []byte("version: 1\nblock:\n  - unbounded-metric-labels\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "metrics.go")
	code := `package main
import "github.com/prometheus/client_golang/prometheus"
var _ = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "x"}, []string{"user_id"})
`
	if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand()
	root.SetArgs([]string{"check", "--policy", policy, "--engine", "pattern", dir})
	err := root.Execute()
	if err == nil {
		t.Fatal("want policy violation error")
	}
	var viol policyViolationError
	if !errors.As(err, &viol) && !strings.Contains(err.Error(), "opted-in blocking") {
		t.Fatalf("want policyViolationError, got %T: %v", err, err)
	}
}

func TestCheckCommandAllowsAdvisoryOnly(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	policy := filepath.Join(dir, "policy.yaml")
	// missing-propagation only — cardinality findings must not fail the build
	if err := os.WriteFile(policy, []byte("version: 1\nblock:\n  - missing-propagation\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(dir, "metrics.go")
	code := `package main
import "github.com/prometheus/client_golang/prometheus"
var _ = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "x"}, []string{"user_id"})
`
	if err := os.WriteFile(src, []byte(code), 0o600); err != nil {
		t.Fatal(err)
	}

	root := NewRootCommand()
	root.SetArgs([]string{"check", "--policy", policy, "--engine", "pattern", "--output", "text", dir})
	if err := root.Execute(); err != nil {
		t.Fatalf("cardinality must be advisory when not opted in: %v", err)
	}
}
