// Package resolve is the thin per-file import/symbol layer for the pattern
// tier. It is deliberately not a type checker: alias->package maps plus
// file-level string / string-slice literals (and Python Exception params).
package resolve

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
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

func analyzeGo(f *File, src []byte) { //nolint:gocyclo // file-level Go parser with several declaration shapes

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, f.Path, src, parser.SkipObjectResolution)
	if err != nil {
		return // ponytail: skip unparsable files; pattern tier still runs
	}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}
		name := defaultImportName(path)
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				continue
			}
			name = imp.Name.Name
		}
		f.Imports[name] = path
	}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
			continue
		}
		for _, spec := range gen.Specs {
			// CONST/VAR GenDecl specs are always *ast.ValueSpec.
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if name == nil || name.Name == "_" {
					continue
				}
				var val ast.Expr
				if i < len(vs.Values) {
					val = vs.Values[i]
				} else if len(vs.Values) == 1 {
					val = vs.Values[0]
				}
				if val == nil {
					continue
				}
				f.Strings[name.Name] = append(f.Strings[name.Name], stringLits(fset, val)...)
			}
		}
	}
}

func defaultImportName(path string) string {
	parts := strings.Split(path, "/")
	name := parts[len(parts)-1]
	if len(parts) > 1 && len(name) > 1 && name[0] == 'v' && name[1] >= '0' && name[1] <= '9' {
		return parts[len(parts)-2]
	}
	return name
}

func stringLits(fset *token.FileSet, expr ast.Expr) []Lit {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return nil
		}
		v, err := strconv.Unquote(e.Value)
		if err != nil {
			return nil
		}
		return []Lit{{Value: v, Line: fset.Position(e.Pos()).Line}}
	case *ast.CompositeLit:
		var out []Lit
		for _, elt := range e.Elts {
			out = append(out, stringLits(fset, elt)...)
		}
		return out
	default:
		return nil
	}
}

var (
	tsFromRE  = regexp.MustCompile(`(?m)^\s*import\s+(?:type\s+)?(?:(\w+)|(?:\* as (\w+))|(?:\{[^}]+\}))\s+from\s+['"]([^'"]+)['"]`)
	tsConstRE = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*(?::[^=]+)?=\s*(\[[^\]]*\]|['"][^'"]*['"])`)
	tsStrRE   = regexp.MustCompile(`['"]([^'"]+)['"]`)
)

func analyzeTS(f *File, src string) {
	for _, m := range tsFromRE.FindAllStringSubmatch(src, -1) {
		path := m[3]
		name := m[1]
		if name == "" {
			name = m[2]
		}
		if name == "" {
			// named-only import: use basename as a weak key for Imports
			base := filepath.Base(path)
			f.Imports[base] = path
			continue
		}
		f.Imports[name] = path
	}
	// side-effect imports
	for _, m := range regexp.MustCompile(`(?m)^\s*import\s+['"]([^'"]+)['"]`).FindAllStringSubmatch(src, -1) {
		path := m[1]
		f.Imports[filepath.Base(path)] = path
	}
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		m := tsConstRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name, rhs := m[1], m[2]
		if strings.HasPrefix(strings.TrimSpace(rhs), "[") {
			continue // multi-line path owns arrays (incl. one-liners below)
		}
		for _, sm := range tsStrRE.FindAllStringSubmatchIndex(rhs, -1) {
			val := rhs[sm[2]:sm[3]]
			f.Strings[name] = append(f.Strings[name], Lit{Value: val, Line: i + 1})
		}
	}
	analyzeMultilineArray(f, src, `(?m)^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*(?::[^=]+)?=\s*\[`)
}

var (
	javaImportRE = regexp.MustCompile(`(?m)^\s*import\s+(?:static\s+)?([\w.]+)\s*;`)
	javaArrRE    = regexp.MustCompile(`(?m)^\s*(?:private|public|protected)?\s*(?:static\s+)?(?:final\s+)?String\s*\[\s*\]\s+(\w+)\s*=\s*\{`)
	javaStrRE    = regexp.MustCompile(`"([^"\\]|\\.)*"`)
)

func analyzeJava(f *File, src string) {
	for _, m := range javaImportRE.FindAllStringSubmatch(src, -1) {
		path := m[1]
		parts := strings.Split(path, ".")
		f.Imports[parts[len(parts)-1]] = path
	}
	analyzeMultilineBraceArray(f, src, javaArrRE)
}

var (
	pyImportRE = regexp.MustCompile(`(?m)^\s*(?:from\s+([\w.]+)\s+import\s+([\w* ,]+)|import\s+([\w.]+)(?:\s+as\s+(\w+))?)`)
	pyFuncRE   = regexp.MustCompile(`(?m)^\s*def\s+\w+\s*\(([^)]*)\)`)
	pyParamRE  = regexp.MustCompile(`(\w+)\s*:\s*([\w.]+)`)
	pyAssignRE = regexp.MustCompile(`(?m)^\s*(\w+)\s*=\s*(\[[^\]]*\]|['"][^'"]*['"])`)
)

func analyzePython(f *File, src string) { //nolint:gocyclo // import + param + assign extraction

	for _, m := range pyImportRE.FindAllStringSubmatch(src, -1) {
		switch {
		case m[1] != "":
			mod := m[1]
			for name := range strings.SplitSeq(m[2], ",") {
				name = strings.TrimSpace(name)
				if name == "" || name == "*" {
					continue
				}
				if before, after, ok := strings.Cut(name, " as "); ok {
					f.Imports[strings.TrimSpace(after)] = mod + "." + strings.TrimSpace(before)
				} else {
					f.Imports[name] = mod + "." + name
				}
			}
		case m[3] != "":
			mod := m[3]
			name := mod
			if i := strings.LastIndex(mod, "."); i >= 0 {
				name = mod[i+1:]
			}
			if m[4] != "" {
				name = m[4]
			}
			f.Imports[name] = mod
		}
	}
	for _, m := range pyFuncRE.FindAllStringSubmatch(src, -1) {
		for _, p := range pyParamRE.FindAllStringSubmatch(m[1], -1) {
			f.Params[p[1]] = p[2]
		}
	}
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		m := pyAssignRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		name, rhs := m[1], m[2]
		for _, sm := range tsStrRE.FindAllStringSubmatchIndex(rhs, -1) {
			f.Strings[name] = append(f.Strings[name], Lit{Value: rhs[sm[2]:sm[3]], Line: i + 1})
		}
	}
}

func analyzeMultilineArray(f *File, src, headerPat string) {
	re := regexp.MustCompile(headerPat)
	for _, loc := range re.FindAllStringSubmatchIndex(src, -1) {
		name := src[loc[2]:loc[3]]
		start := loc[1]
		end := strings.Index(src[start:], "]")
		if end < 0 {
			continue
		}
		body := src[start : start+end]
		baseLine := strings.Count(src[:start], "\n") + 1
		for _, sm := range tsStrRE.FindAllStringSubmatchIndex(body, -1) {
			line := baseLine + strings.Count(body[:sm[0]], "\n")
			f.Strings[name] = append(f.Strings[name], Lit{Value: body[sm[2]:sm[3]], Line: line})
		}
	}
}

func analyzeMultilineBraceArray(f *File, src string, header *regexp.Regexp) {
	for _, loc := range header.FindAllStringSubmatchIndex(src, -1) {
		name := src[loc[2]:loc[3]]
		start := loc[1]
		end := strings.Index(src[start:], "}")
		if end < 0 {
			continue
		}
		body := src[start : start+end]
		baseLine := strings.Count(src[:start], "\n") + 1
		for _, sm := range javaStrRE.FindAllStringIndex(body, -1) {
			raw := body[sm[0]:sm[1]]
			v, err := strconv.Unquote(raw)
			if err != nil {
				v = strings.Trim(raw, `"`)
			}
			line := baseLine + strings.Count(body[:sm[0]], "\n")
			f.Strings[name] = append(f.Strings[name], Lit{Value: v, Line: line})
		}
	}
}
