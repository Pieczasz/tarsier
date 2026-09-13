// Package finding defines the normalized finding schema every analyzer emits
// and every consumer (report, store, PR check, AI layer) reads.
package finding

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// Finding lifecycle states. Baselines drop known findings from output;
// suppressions keep them, marked.
const (
	// StatusOpen is a finding reported as-is.
	StatusOpen = "open"
	// StatusSuppressed is a finding quieted by an inline tarsier:ignore
	// comment. It stays in JSON output so consumers can audit suppressions.
	StatusSuppressed = "suppressed"
)

// Finding is a single normalized observability gap emitted by an analyzer.
type Finding struct {
	Fingerprint string `json:"fingerprint"`
	Rule        string `json:"rule"`
	Language    string `json:"language"`
	Severity    string `json:"severity"`
	Confidence  string `json:"confidence"`

	Location Location `json:"location"`
	Message  string   `json:"message"`
	Note     string   `json:"note,omitempty"`
	Evidence Evidence `json:"evidence"`

	WorkflowRefs []WorkflowRef `json:"workflow_refs,omitempty"`

	Impact             string `json:"impact,omitempty"`
	RecommendationType string `json:"recommendation_type,omitempty"`
	EstimatedEffort    string `json:"estimated_effort,omitempty"`
	Status             string `json:"status,omitempty"`
}

// Location pins a finding to a file, line, and symbol.
type Location struct {
	// Module is an input-side grouping key used in Fingerprint. It is omitted
	// from JSON so report consumers see the on-disk path, not scanner grouping.
	Module string `json:"-"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Symbol string `json:"symbol,omitempty"`
}

// Evidence carries the static and runtime proof behind a finding.
type Evidence struct {
	Static  map[string]any `json:"static,omitempty"`
	Runtime map[string]any `json:"runtime,omitempty"`
}

// WorkflowRef links a finding to a named business-critical workflow step.
type WorkflowRef struct {
	Workflow    string `json:"workflow"`
	Step        string `json:"step"`
	Criticality string `json:"criticality"`
}

// Fingerprint hashes the finding identity for baselines and suppression.
func Fingerprint(rule, module, relPath, symbol string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		rule, module, strings.ReplaceAll(relPath, "\\", "/"), symbol,
	}, "|")))
	return hex.EncodeToString(sum[:])
}

// Fill computes and sets the finding fingerprint from its identity fields.
func (f *Finding) Fill() {
	f.Fingerprint = Fingerprint(f.Rule, f.Location.Module, f.Location.File, f.Location.Symbol)
}
