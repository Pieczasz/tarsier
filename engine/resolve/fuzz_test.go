package resolve

import (
	"testing"
	"unicode/utf8"
)

func FuzzAnalyzeAnyLang(f *testing.F) {
	f.Add("a.go", []byte("package main\nconst x = \"user_id\"\n"))
	f.Add("a.ts", []byte("import x from 'prom';\nconst labels = ['user_id'];\n"))
	f.Add("A.java", []byte("import io.micrometer.core.instrument.Tags;\nString[] L = {\"user_id\"};\n"))
	f.Add("a.py", []byte("from prometheus_client import Counter\nx = ['user_id']\n"))
	f.Add("a.rs", []byte("fn main() {}\n"))
	f.Add("a.go", []byte("package main\nconst (\n  a = \"user_id\"\n  b\n)\n"))
	f.Add("weird.xyz", []byte("not a language"))
	f.Add("a.ts", []byte("const x = [\n  'a',\n"))
	f.Fuzz(func(t *testing.T, path string, src []byte) {
		if len(path) > 64 || len(src) > 8*1024 {
			return
		}
		if !utf8.ValidString(path) {
			return
		}
		file := Analyze(path, src)
		if file == nil {
			t.Fatal("Analyze must never return nil")
		}
		if file.Imports == nil || file.Strings == nil || file.Params == nil {
			t.Fatal("maps must be non-nil")
		}
		_ = file.HasImport("otel")
		_ = file.PackageOf("x")
		for name, lits := range file.Strings {
			if name == "" {
				t.Fatal("empty string key")
			}
			for _, lit := range lits {
				if lit.Line < 1 {
					t.Fatalf("line < 1 for %q", name)
				}
			}
		}
	})
}

func FuzzDefaultImportName(f *testing.F) {
	f.Add("github.com/foo/bar")
	f.Add("github.com/foo/bar/v2")
	f.Add("v2")
	f.Add("")
	f.Add("a/b/c")
	f.Add("x/v")
	f.Fuzz(func(t *testing.T, path string) {
		if len(path) > 256 {
			return
		}
		a := defaultImportName(path)
		if defaultImportName(path) != a {
			t.Fatal("unstable")
		}
	})
}

func FuzzUnboundedLabel(f *testing.F) {
	f.Add("user_id")
	f.Add("USER_ID")
	f.Add(`"email"`)
	f.Add("tenant_id")
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) {
		if utf8.RuneCountInString(s) > 64 {
			return
		}
		_ = UnboundedLabel(s)
	})
}
