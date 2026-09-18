package finding

import "testing"

func FuzzFingerprintFieldBoundaries(f *testing.F) {
	f.Add("rule", "mod", "file.go", "sym")
	f.Add("a|b", "", "x", "")
	f.Add("r", "", "a|b", "c")
	f.Add("", "", "", "")
	f.Add("r", "m", `path\with\backslash`, "s|ym")
	f.Fuzz(func(t *testing.T, rule, module, file, symbol string) {
		a := Fingerprint(rule, module, file, symbol)
		if Fingerprint(rule, module, file, symbol) != a {
			t.Fatal("unstable fingerprint")
		}
		// Pipe inside file vs symbol must not share a digest.
		if Fingerprint(rule, module, "a|b", "c") == Fingerprint(rule, module, "a", "b|c") {
			t.Fatal("file/symbol boundary collision")
		}
		if Fingerprint(rule+"|x", module, file, symbol) == Fingerprint(rule, "x|"+module, file, symbol) {
			t.Fatal("rule/module boundary collision")
		}
	})
}
