package finding

import (
	"encoding/hex"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzFingerprintStable(f *testing.F) {
	f.Add("rule", "mod", "file.go", "sym")
	f.Add("a|b", "", "x", "")
	f.Add("", "", "", "")
	f.Add("r", "m", `path\with\slash`, "s")
	f.Fuzz(func(t *testing.T, rule, module, file, symbol string) {
		a := Fingerprint(rule, module, file, symbol)
		if Fingerprint(rule, module, file, symbol) != a {
			t.Fatal("unstable fingerprint")
		}
		if len(a) != hex.EncodedLen(32) {
			t.Fatalf("len=%d, want sha256 hex", len(a))
		}
		// Slash normalization: backslash and slash in file must collide.
		if strings.Contains(file, `\`) {
			slashy := strings.ReplaceAll(file, `\`, "/")
			if Fingerprint(rule, module, slashy, symbol) != a {
				t.Fatal("backslash path must fingerprint like slash path")
			}
		}
	})
}

func FuzzParseSuppressions(f *testing.F) {
	f.Add([]byte("// tarsier:ignore metrics/x because reasons\n"))
	f.Add([]byte("# tarsier:ignore metrics/x ok\ncode()\n"))
	f.Add([]byte("-- tarsier:ignore metrics/x ok\n"))
	f.Add([]byte("// tarsier:ignore metrics/x\n")) // no reason
	f.Add([]byte("code // tarsier:ignore metrics/x trailing\n"))
	f.Add([]byte(""))
	f.Add([]byte(strings.Repeat("x", 1000) + "\n// tarsier:ignore r reason\n"))
	f.Fuzz(func(t *testing.T, src []byte) {
		if len(src) > 64*1024 {
			return
		}
		out := ParseSuppressions("a.go", src)
		prev := 0
		for _, s := range out {
			if s.File != "a.go" {
				t.Fatal("file must be the path argument")
			}
			if s.Line < 1 {
				t.Fatal("line must be 1-based")
			}
			if s.Line < prev {
				t.Fatal("lines must be non-decreasing")
			}
			prev = s.Line
			if s.Rule == "" || strings.TrimSpace(s.Reason) == "" {
				t.Fatal("rule and reason required")
			}
		}
		kept, supp := ApplySuppressions(nil, out)
		if len(kept) != 0 || len(supp) != 0 {
			t.Fatal("nil findings must stay empty")
		}
	})
}

func FuzzPartitionNew(f *testing.F) {
	f.Add("fp1", true, "fp2", false)
	f.Add("fp1", true, "fp1", true)
	f.Add("", false, "x", true)
	f.Fuzz(func(t *testing.T, a string, aKnown bool, b string, bKnown bool) {
		if utf8.RuneCountInString(a) > 64 || utf8.RuneCountInString(b) > 64 {
			return
		}
		findings := []Finding{{Fingerprint: a}, {Fingerprint: b}}
		var baseline map[string]bool
		if aKnown || bKnown {
			baseline = map[string]bool{}
			if aKnown {
				baseline[a] = true
			}
			if bKnown {
				baseline[b] = true
			}
		}
		fresh, known := PartitionNew(findings, baseline)
		if len(fresh)+len(known) != len(findings) {
			t.Fatalf("fresh(%d)+known(%d) != %d", len(fresh), len(known), len(findings))
		}
		for i := range fresh {
			if baseline[fresh[i].Fingerprint] {
				t.Fatal("fresh finding is in baseline")
			}
		}
		for i := range known {
			if baseline == nil || !baseline[known[i].Fingerprint] {
				t.Fatal("known finding not in baseline")
			}
		}
	})
}
