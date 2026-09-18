package finding

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBaselineRoundTripPreservesTheSet(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "baseline.json")

	in := []Finding{
		{Rule: "a", Location: Location{File: "x.go", Line: 1, Symbol: "s1"}},
		{Rule: "b", Location: Location{File: "y.go", Line: 2, Symbol: "s2"}},
	}
	for i := range in {
		in[i].Fill()
	}
	if err := WriteBaseline(path, in); err != nil {
		t.Fatal(err)
	}

	got, err := LoadBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[in[0].Fingerprint] || !got[in[1].Fingerprint] {
		t.Fatalf("round trip lost fingerprints: %v", got)
	}
}

func TestPartitionNewSplitsKnownFromNew(t *testing.T) {
	t.Parallel()

	old := Finding{Rule: "a", Location: Location{File: "x.go", Line: 1, Symbol: "s"}}
	old.Fill()
	fresh := Finding{Rule: "a", Location: Location{File: "x.go", Line: 9, Symbol: "other"}}
	fresh.Fill()

	baseline := map[string]bool{old.Fingerprint: true}
	newFindings, known := PartitionNew([]Finding{old, fresh}, baseline)

	if len(newFindings) != 1 || newFindings[0].Fingerprint != fresh.Fingerprint {
		t.Fatalf("new set wrong: %+v", newFindings)
	}
	if len(known) != 1 || known[0].Fingerprint != old.Fingerprint {
		t.Fatalf("known set wrong: %+v", known)
	}
}

func TestPartitionNewWithNilBaselineReportsEverything(t *testing.T) {
	t.Parallel()

	f := Finding{Rule: "a", Location: Location{File: "x.go", Line: 1, Symbol: "s"}}
	f.Fill()

	newFindings, known := PartitionNew([]Finding{f}, nil)
	if len(newFindings) != 1 || len(known) != 0 {
		t.Fatalf("no baseline must report everything as new, got %+v / %+v", newFindings, known)
	}
}

func TestLoadBaselineRejectsUnknownVersion(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "baseline.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"fingerprints":["abc"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBaseline(path); err == nil {
		t.Fatal("a future baseline version must error, not silently parse")
	}
}

func TestLoadBaselineRejectsV1Fingerprints(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "baseline.json")
	// v1 used pipe-joined SHA-256; v2 length-prefixes fields. Refuse v1 so
	// CI does not silently treat every finding as new.
	if err := os.WriteFile(path, []byte(`{"version":1,"fingerprints":["abc"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBaseline(path); err == nil {
		t.Fatal("v1 baselines must error so operators regenerate with --baseline-write")
	}
}

func TestLoadBaselineMissingPathIsAnError(t *testing.T) {
	t.Parallel()

	if _, err := LoadBaseline(filepath.Join(t.TempDir(), "no-such.json")); err == nil {
		t.Fatal("a typo'd baseline path must error, not silently report everything as new")
	}
}
