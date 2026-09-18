// Package resolve is the thin per-file import/symbol layer for the pattern
// tier. It is deliberately not a type checker: alias->package maps plus
// file-level string / string-slice literals (and Python Exception params).
package resolve

import (
	"path/filepath"
	"strings"
)

// Lit is a string literal with its 1-based source line.
type Lit struct {
	Value string
	Line  int
}

// File holds the facts one source file yields.
type File struct {
	Path    string
	Lang    string
	Imports map[string]string // local name -> import path
	Strings map[string][]Lit  // const/var name -> string literals (incl. slice elems)
	Params  map[string]string // Python: param -> type annotation text
}

const (
	langGo         = "go"
	langTypeScript = "typescript"
	langJava       = "java"
	langPython     = "python"
)

// LangOf maps a path extension to a language tag. Empty means unsupported.
func LangOf(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go":
		return langGo
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs":
		return langTypeScript
	case ".java":
		return langJava
	case ".py":
		return langPython
	default:
		return ""
	}
}

// Analyze extracts imports and file-level string facts. Unknown languages
// return an empty File without error.
func Analyze(path string, src []byte) *File {
	f := &File{
		Path:    path,
		Lang:    LangOf(path),
		Imports: map[string]string{},
		Strings: map[string][]Lit{},
		Params:  map[string]string{},
	}
	switch f.Lang {
	case langGo:
		analyzeGo(f, src)
	case langTypeScript:
		analyzeTS(f, string(src))
	case langJava:
		analyzeJava(f, string(src))
	case langPython:
		analyzePython(f, string(src))
	}
	return f
}

// PackageOf returns the import path for a local name, or "" if unknown.
func (f *File) PackageOf(name string) string {
	if f == nil {
		return ""
	}
	return f.Imports[name]
}

// HasImport reports whether any import path contains substr.
func (f *File) HasImport(substr string) bool {
	if f == nil {
		return false
	}
	for _, p := range f.Imports {
		if strings.Contains(p, substr) {
			return true
		}
	}
	return false
}

// UnboundedLabel mirrors ruleutils/unbounded-label.yml.
func UnboundedLabel(s string) bool {
	s = strings.Trim(s, `"'`)
	switch strings.ToLower(s) {
	case "user_id", "userid", "email", "uuid", "request_id", "session_id",
		"trace_id", "span_id", "order_id", "ip", "ip_address", "url", "token":
		return true
	default:
		return false
	}
}
