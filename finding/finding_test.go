package finding

import (
	"encoding/json"
	"testing"
)

func TestFingerprintIsLineIndependentAndStable(t *testing.T) {
	a := Finding{
		Rule:     "msgtrace/kafka-produce-no-inject",
		Location: Location{Module: "api", File: "internal/orders/publish.go", Line: 42, Symbol: "PublishOrderCreated"},
	}
	b := a
	b.Location.Line = 907

	a.Fill()
	b.Fill()
	if a.Fingerprint != b.Fingerprint {
		t.Fatal("fingerprint changed with line number; baselines would break on every edit")
	}

	win := Fingerprint(a.Rule, "api", `internal\orders\publish.go`, "PublishOrderCreated")
	if win != a.Fingerprint {
		t.Fatal("path separators leak into the fingerprint")
	}

	if Fingerprint("a|b", "", "x", "") == Fingerprint("a", "b", "x", "") {
		t.Fatal("fingerprint collides across field boundaries")
	}
	if Fingerprint("r", "", "a|b", "c") == Fingerprint("r", "", "a", "b|c") {
		t.Fatal("fingerprint collides across file/symbol boundary")
	}
}

func TestFingerprintIgnoresLineForLongSymbolsToo(t *testing.T) {
	symbol := `kafka.Message{ Topic: "orders", Value: []byte(orderID), }`
	a := Finding{
		Rule:     "msgtrace/kafka-produce-no-inject",
		Location: Location{File: "api/publish.go", Line: 14, Symbol: symbol},
	}
	b := a
	b.Location.Line = 200

	a.Fill()
	b.Fill()
	if a.Fingerprint != b.Fingerprint {
		t.Fatal("fingerprint changed with line number; baselines would break on every edit")
	}
}

func TestFingerprintChangesOnFileRename_Documented(t *testing.T) {
	a := Finding{
		Rule:     "msgtrace/kafka-produce-no-inject",
		Location: Location{File: "api/publish.go", Line: 14, Symbol: "kafka.Message{...}"},
	}
	b := a
	b.Location.File = "api/orders.go"

	a.Fill()
	b.Fill()
	if a.Fingerprint == b.Fingerprint {
		t.Fatal("fingerprint survived a file rename; baselines silently carry over - this is the documented rename-changes contract, do not 'fix' it without updating TAR-21")
	}
}

func TestFindingRoundTripsJSON(t *testing.T) {
	in := Finding{
		Rule:     "metrics/high-cardinality-label",
		Language: "go",
		Severity: "high",
		Location: Location{Module: "api", File: "metrics.go", Line: 7},
		Evidence: Evidence{Runtime: map[string]any{"series": 40000.0}},
	}
	in.Fill()

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Finding
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Fingerprint != in.Fingerprint || out.Location.File != in.Location.File {
		t.Fatalf("round trip lost data: %+v", out)
	}
}
