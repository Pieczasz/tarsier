package pattern

import "testing"

func FuzzVersionAtLeast(f *testing.F) {
	f.Add("0.45.0", "0.45.0")
	f.Add("0.45.1-rc.1", "0.45.0")
	f.Add("v1.2.3", "1.2.0")
	f.Add("", "0.1.0")
	f.Add("10.0.0", "9.99.99")
	f.Fuzz(func(t *testing.T, got, want string) {
		_ = versionAtLeast(got, want)
		// Reflexivity: equal numeric prefixes compare equal.
		if versionAtLeast(got, got) != true {
			t.Fatalf("versionAtLeast(%q,%q) must be true", got, got)
		}
	})
}
