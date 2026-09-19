package cli

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Pieczasz/tarsier/finding"
)

func FuzzBlockingCount(f *testing.F) {
	f.Add("warning", "high", "", 3)
	f.Add("error", "medium", "", 3)
	f.Add("warning", "high", finding.StatusSuppressed, 3)
	f.Add("info", "high", "", 2)
	f.Add("nope", "high", "", 1)
	f.Add("error", "HIGH", "", 4)
	f.Fuzz(func(t *testing.T, sev, conf, status string, threshold int) {
		if utf8.RuneCountInString(sev) > 16 || utf8.RuneCountInString(conf) > 16 {
			return
		}
		if threshold < -2 || threshold > 10 {
			return
		}
		findings := []finding.Finding{{
			Severity:   sev,
			Confidence: conf,
			Status:     status,
		}}
		n := blockingCount(findings, threshold)
		if n < 0 || n > len(findings) {
			t.Fatalf("n=%d out of range", n)
		}
		if threshold <= 0 && n != 0 {
			t.Fatal("threshold<=0 must count 0")
		}
		// Monotonic only in the gating range; 0 is a special "off" switch.
		if threshold > 0 {
			higher := blockingCount(findings, threshold+1)
			if higher > n {
				t.Fatal("raising threshold must not increase count")
			}
		}
	})
}

func FuzzPolicyBlockingCount(f *testing.F) {
	f.Add("metrics/high-cardinality-label", "high", "", true)
	f.Add("metrics/high-cardinality-label", "medium", "", true)
	f.Add("metrics/high-cardinality-label", "high", finding.StatusSuppressed, true)
	f.Add("otel/x", "high", "", true)
	f.Add("metrics/high-cardinality-label", "high", "", false)
	f.Fuzz(func(t *testing.T, rule, conf, status string, inBlocked bool) {
		if utf8.RuneCountInString(rule) > 64 {
			return
		}
		blocked := map[string]bool{}
		if inBlocked {
			blocked[rule] = true
		}
		findings := []finding.Finding{{Rule: rule, Confidence: conf, Status: status}}
		n := policyBlockingCount(findings, blocked)
		if n < 0 || n > 1 {
			t.Fatalf("n=%d", n)
		}
		if len(blocked) == 0 && n != 0 {
			t.Fatal("empty blocked => 0")
		}
		if n == 1 {
			if !blocked[rule] || strings.EqualFold(conf, "medium") || status == finding.StatusSuppressed {
				t.Fatal("counted a finding that should be skipped")
			}
		}
	})
}

func FuzzMergeLifecycle(f *testing.F) {
	f.Add("a", "b", "a", true, false)
	f.Add("a", "a", "b", true, true)
	f.Add("x", "y", "z", false, false)
	f.Fuzz(func(t *testing.T, o1, o2, o3 string, freshFirst, suppSecond bool) {
		if len(o1) > 8 || len(o2) > 8 || len(o3) > 8 {
			return
		}
		original := []finding.Finding{
			{Fingerprint: o1},
			{Fingerprint: o2},
			{Fingerprint: o3},
		}
		var fresh, supp []finding.Finding
		if freshFirst {
			fresh = append(fresh, finding.Finding{Fingerprint: o1, Status: finding.StatusOpen})
		}
		if suppSecond {
			supp = append(supp, finding.Finding{Fingerprint: o2, Status: finding.StatusSuppressed})
		}
		out := mergeLifecycle(original, fresh, supp)
		emit := map[string]bool{}
		for i := range fresh {
			emit[fresh[i].Fingerprint] = true
		}
		for i := range supp {
			emit[supp[i].Fingerprint] = true
		}
		// out must be a subsequence of original (order preserved, duplicates OK).
		j := 0
		for i := range out {
			if !emit[out[i].Fingerprint] {
				t.Fatal("output fingerprint not in fresh∪suppressed")
			}
			for j < len(original) && original[j].Fingerprint != out[i].Fingerprint {
				j++
			}
			if j >= len(original) {
				t.Fatal("output is not a subsequence of original")
			}
			j++
		}
	})
}
