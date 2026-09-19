package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/Pieczasz/tarsier/finding"
)

// Opt-in blocking classes (TAR-47). Only these may appear under policy.block.
// Each maps to rule IDs (metadata.rule). Empty rule lists are reserved slots.
var blockableClasses = map[string][]string{
	"unbounded-metric-labels": {
		"metrics/high-cardinality-label",
		"metrics/unbounded-label-value",
	},
	"secrets-in-logs": {
		"logs/secrets-in-logs",
	},
	"missing-propagation": {
		"msgtrace/kafka-produce-no-inject",
		"msgtrace/kafka-consume-no-extract",
	},
	"removed-required-events": {
		// reserved: required business events removed from critical path
	},
}

// nonBlockableRules are expectation-layer 3/4 rules that must never be named
// as blocking entries (parse-time reject), even if someone invents a class.
var nonBlockableRules = map[string]int{
	"logs/unstructured-logging": 4,
}

// Policy is the on-disk observability-policy.yaml schema.
type Policy struct {
	Version int      `yaml:"version"`
	Block   []string `yaml:"block"`
}

// policyError is a validated-policy failure (bad schema or illegal block entry).
type policyError struct {
	msg string
}

func (e policyError) Error() string { return e.msg }

// LoadPolicy reads and validates a policy file.
func LoadPolicy(path string) (*Policy, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // G304: explicit --policy path
	if err != nil {
		return nil, fmt.Errorf("read policy %s: %w", path, err)
	}
	var p Policy
	if err := yaml.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("decode policy %s: %w", path, err)
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Policy) validate() error {
	if p.Version != 1 {
		return policyError{msg: fmt.Sprintf("policy version %d, want 1", p.Version)}
	}
	if len(p.Block) == 0 {
		return policyError{msg: "policy.block is empty; list at least one opt-in class or omit --policy"}
	}
	seen := map[string]bool{}
	normalized := make([]string, 0, len(p.Block))
	for _, entry := range p.Block {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return policyError{msg: "policy.block contains an empty entry"}
		}
		if seen[entry] {
			return policyError{msg: fmt.Sprintf("policy.block duplicates %q", entry)}
		}
		seen[entry] = true
		if layer, bad := nonBlockableRules[entry]; bad {
			return policyError{msg: fmt.Sprintf(
				"policy.block %q is expectation layer %d and cannot be blocking; allowed classes: %s",
				entry, layer, allowedBlockClasses())}
		}
		if _, ok := blockableClasses[entry]; !ok {
			return policyError{msg: fmt.Sprintf(
				"policy.block %q is not an opt-in blocking class; allowed: %s",
				entry, allowedBlockClasses())}
		}
		normalized = append(normalized, entry)
	}
	p.Block = normalized // enforce uses the same strings validate accepted
	return nil
}

func allowedBlockClasses() string {
	names := make([]string, 0, len(blockableClasses))
	for name := range blockableClasses {
		names = append(names, name)
	}
	// stable-ish for error messages
	slices.Sort(names)
	return strings.Join(names, ", ")
}

// blockedRules expands opted-in classes to rule IDs.
func (p *Policy) blockedRules() map[string]bool {
	out := map[string]bool{}
	for _, class := range p.Block {
		for _, rule := range blockableClasses[class] {
			out[rule] = true
		}
	}
	return out
}

// policyViolationError is returned when check finds opted-in blocking findings.
type policyViolationError struct {
	count int
}

func (e policyViolationError) Error() string {
	return fmt.Sprintf("%d findings match opted-in blocking classes", e.count)
}

// policyBlockingCount counts non-suppressed findings whose rule is blocked.
// Medium-confidence findings never gate (same as --fail-on).
func policyBlockingCount(findings []finding.Finding, blocked map[string]bool) int {
	if len(blocked) == 0 {
		return 0
	}
	n := 0
	for i := range findings {
		f := &findings[i]
		if f.Status == finding.StatusSuppressed {
			continue
		}
		if strings.EqualFold(f.Confidence, "medium") {
			continue
		}
		if blocked[f.Rule] {
			n++
		}
	}
	return n
}
