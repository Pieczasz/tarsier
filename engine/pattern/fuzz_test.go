package pattern

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzParseVersionAtLeast(f *testing.F) {
	f.Add("0.45.0", "0.45.0")
	f.Add("0.45.1", "0.45.0")
	f.Add("0.44.9", "0.45.0")
	f.Add("v1.2.3", "1.2.3")
	f.Add("1-rc.1", "1.0.0")
	f.Add("", "")
	f.Add("abc", "0.0.1")
	f.Fuzz(func(t *testing.T, got, want string) {
		if utf8.RuneCountInString(got) > 32 || utf8.RuneCountInString(want) > 32 {
			return
		}
		_ = parseVersion(got)
		_ = parseVersion(want)
		a := versionAtLeast(got, want)
		b := versionAtLeast(got, want)
		if a != b {
			t.Fatal("unstable versionAtLeast")
		}
		// Reflexive: any version is at least itself.
		if !versionAtLeast(got, got) {
			t.Fatal("version must be >= itself")
		}
	})
}

func FuzzTruncateBase(f *testing.F) {
	f.Add("")
	f.Add("short")
	f.Add(strings.Repeat("a", 200))
	f.Add(strings.Repeat("界", 80))
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 8*1024 {
			return
		}
		out := truncateBase(s)
		if utf8.RuneCountInString(out) > 120 {
			t.Fatalf("truncateBase len=%d runes", utf8.RuneCountInString(out))
		}
		if truncateBase(s) != out {
			t.Fatal("unstable")
		}
	})
}

func FuzzExcludedRelPath(f *testing.F) {
	f.Add("foo.go")
	f.Add("foo-test-bar/x.go")
	f.Add("vendor/x.go")
	f.Add(`a\b\c.go`)
	f.Add(".")
	f.Add("")
	f.Fuzz(func(t *testing.T, path string) {
		if len(path) > 512 {
			return
		}
		_ = excluded(path)
		_ = excluded(relPath("/tmp/root", path))
		// Must not panic on repeated calls.
		_ = excluded(path)
	})
}

func FuzzDecodeAstGrepJSON(f *testing.F) {
	valid, _ := json.Marshal(map[string]any{
		"ruleId":   "metrics-high-cardinality-label-go",
		"file":     "a.go",
		"severity": "warning",
		"message":  "hit",
		"text":     "user_id",
		"range":    map[string]any{"start": map[string]any{"line": 1}},
		"metadata": map[string]any{"rule": "metrics/high-cardinality-label", "confidence": "high"},
	})
	f.Add(append(valid, '\n'))
	f.Add([]byte(""))
	f.Add([]byte("{"))
	f.Add([]byte("null\n"))
	f.Add(bytes.Repeat([]byte("x"), 100))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 16*1024 {
			return
		}
		out, err := decode(bytes.NewReader(raw), "/tmp", "/tmp")
		if err == nil {
			for i := range out {
				if out[i].Fingerprint == "" {
					t.Fatal("finding missing fingerprint")
				}
			}
		}
		// Must not panic on second pass of same bytes.
		_, _ = decode(bytes.NewReader(raw), "/tmp", "/tmp")
		_, _ = io.Copy(io.Discard, bytes.NewReader(raw))
	})
}
