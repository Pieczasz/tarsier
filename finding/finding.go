// Package finding defines the normalized finding schema every analyzer emits
// and every consumer (report, store, PR check, AI layer) reads.
package finding

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"strings"
)

// Finding lifecycle states. Baselines drop known findings from output;
// suppressions keep them, marked.
const (
	StatusOpen = "open"
	// StatusSuppressed stays in JSON so consumers can audit quieted findings.
	StatusSuppressed = "suppressed"
)

// Finding is the shared schema across analyzers, reports, store, and PR checks.
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

// Fingerprint is stable across line drift; baselines and suppressions key on it.
// Fields are length-prefixed so a "|" (or any byte) inside one field cannot
// collide with a boundary in another (rule|module vs file|symbol).
func Fingerprint(rule, module, relPath, symbol string) string {
	h := sha256.New()
	writeFP(h, rule)
	writeFP(h, module)
	writeFP(h, strings.ReplaceAll(relPath, "\\", "/"))
	writeFP(h, symbol)
	return hex.EncodeToString(h.Sum(nil))
}

func writeFP(h hash.Hash, s string) {
	var b [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(b[:], uint64(len(s)))
	_, _ = h.Write(b[:n])
	_, _ = h.Write([]byte(s))
}

// Fill computes and sets the finding fingerprint from its identity fields.
func (f *Finding) Fill() {
	f.Fingerprint = Fingerprint(f.Rule, f.Location.Module, f.Location.File, f.Location.Symbol)
}
