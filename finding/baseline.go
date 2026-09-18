package finding

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

// baselineVersion 2: fingerprints are length-prefixed (see finding.Fingerprint).
// Version 1 files must be regenerated with --baseline-write.
const baselineVersion = 2

// Baseline is the versioned fingerprint set recorded at adoption. A later
// scan reports only findings whose fingerprint is absent here.
type Baseline struct {
	Version      int      `json:"version"`
	Fingerprints []string `json:"fingerprints"`
}

// WriteBaseline records the fingerprint set of findings at path, sorted so
// the file diffs stably. Fingerprints of private codebases get 0o600.
func WriteBaseline(path string, findings []Finding) error {
	set := make(map[string]bool, len(findings))
	for i := range findings {
		set[findings[i].Fingerprint] = true
	}
	fps := make([]string, 0, len(set))
	for fp := range set {
		fps = append(fps, fp)
	}
	slices.Sort(fps)

	raw, err := json.MarshalIndent(Baseline{Version: baselineVersion, Fingerprints: fps}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode baseline %s: %w", path, err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		return fmt.Errorf("write baseline %s: %w", path, err)
	}
	return nil
}

// LoadBaseline reads a baseline file. A missing or corrupt path is an error,
// never an empty set: silently reporting everything as new is worse than
// refusing to scan.
func LoadBaseline(path string) (map[string]bool, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // G304: path is an explicit CLI flag, not remote input
	if err != nil {
		return nil, fmt.Errorf("read baseline %s: %w", path, err)
	}
	var base Baseline
	if err := json.Unmarshal(raw, &base); err != nil {
		return nil, fmt.Errorf("decode baseline %s: %w", path, err)
	}
	if base.Version != baselineVersion {
		return nil, fmt.Errorf("decode baseline %s: version %d, want %d", path, base.Version, baselineVersion)
	}
	set := make(map[string]bool, len(base.Fingerprints))
	for _, fp := range base.Fingerprints {
		set[fp] = true
	}
	return set, nil
}

// PartitionNew splits findings into fresh (absent from baseline) and known. A
// nil baseline reports everything as fresh.
func PartitionNew(findings []Finding, baseline map[string]bool) (fresh, known []Finding) {
	for i := range findings {
		if baseline[findings[i].Fingerprint] {
			known = append(known, findings[i])
		} else {
			fresh = append(fresh, findings[i])
		}
	}
	return fresh, known
}
