package cli

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzPolicyValidateThenBlockedRules checks the property that caught the
// whitespace bypass: any Policy that validate() accepts must expand via
// blockedRules() using the same normalized class names.
func FuzzPolicyValidateThenBlockedRules(f *testing.F) {
	for class := range blockableClasses {
		f.Add(class, "", "")
		f.Add(class, "  ", "  ")
		f.Add(class, "\t", "\n")
		f.Add("  "+class+"  ", "", "")
	}
	f.Add("logs/unstructured-logging", " ", " ")
	f.Add("not-a-real-class", "", "")
	f.Add("", "  ", "  ")
	f.Add("unbounded-metric-labels", "\x00", "")

	f.Fuzz(func(t *testing.T, class, prefix, suffix string) {
		// Keep pads small / printable-ish so we exercise TrimSpace, not YAML.
		if utf8.RuneCountInString(prefix) > 8 || utf8.RuneCountInString(suffix) > 8 {
			return
		}
		entry := prefix + class + suffix
		p := &Policy{Version: 1, Block: []string{entry}}
		err := p.validate()
		trimmed := strings.TrimSpace(entry)
		switch {
		case trimmed == "":
			if err == nil {
				t.Fatal("empty/whitespace-only entry must fail validate")
			}
		case nonBlockableRules[trimmed] != 0:
			if err == nil {
				t.Fatal("layer 3/4 rule must fail validate")
			}
		case blockableClasses[trimmed] == nil:
			if err == nil {
				t.Fatalf("unknown class %q must fail validate", trimmed)
			}
		default:
			if err != nil {
				t.Fatalf("known class %q: validate: %v", trimmed, err)
			}
			assertPolicyEnforceable(t, p)
			if len(p.Block) != 1 || p.Block[0] != trimmed {
				t.Fatalf("Block=%v, want [%q]", p.Block, trimmed)
			}
		}
	})
}
