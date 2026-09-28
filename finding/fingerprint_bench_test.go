package finding

import "testing"

func BenchmarkFingerprint(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = Fingerprint("metrics/high-cardinality-label", "api", "internal/orders/metrics.go", `prometheus.Labels{"user_id"}`)
	}
}

func BenchmarkApplySuppressions(b *testing.B) {
	findings := make([]Finding, 64)
	for i := range findings {
		findings[i] = Finding{
			Rule:     "rule/a",
			Location: Location{File: "f.go", Line: i + 1, Symbol: "s"},
			Status:   StatusOpen,
		}
		findings[i].Fill()
	}
	sup := []Suppression{
		{File: "f.go", Line: 1, Rule: "rule/a", Reason: "ok"},
		{File: "f.go", Line: 32, Rule: "rule/a", Reason: "ok"},
	}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = ApplySuppressions(findings, sup)
	}
}
