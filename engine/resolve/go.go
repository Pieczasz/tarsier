package resolve

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

func analyzeGo(f *File, src []byte) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, f.Path, src, parser.SkipObjectResolution)
	if err != nil {
		return // ponytail: skip unparsable files; pattern tier still runs
	}
	collectGoImports(f, file)
	collectGoStringDecls(f, fset, file)
}

func collectGoImports(f *File, file *ast.File) {
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
}

func collectGoStringDecls(f *File, fset *token.FileSet, file *ast.File) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || (gen.Tok != token.CONST && gen.Tok != token.VAR) {
			continue
		}
		// Const blocks may omit the expression list; Go reuses the previous
		// non-empty list. Mirror that so label names in carried specs resolve.
		var lastValues []ast.Expr
		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			values := vs.Values
			if gen.Tok == token.CONST {
				if len(values) == 0 {
					values = lastValues
				} else {
					lastValues = values
				}
			}
			bindGoNames(f, fset, vs.Names, values)
		}
	}
}

func bindGoNames(f *File, fset *token.FileSet, names []*ast.Ident, values []ast.Expr) {
	for i, name := range names {
		if name == nil || name.Name == "_" {
			continue
		}
		var val ast.Expr
		if i < len(values) {
			val = values[i]
		} else if len(values) == 1 {
			val = values[0]
		}
		if val == nil {
			continue
		}
		f.Strings[name.Name] = append(f.Strings[name.Name], stringLits(fset, val)...)
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
