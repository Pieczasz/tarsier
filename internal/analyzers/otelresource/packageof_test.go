package otelresource

import (
	"go/ast"
	"go/token"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestPackageOfNestedSelectorAndCalleeNil(t *testing.T) {
	pass := &analysis.Pass{TypesInfo: &types.Info{
		Defs:  map[*ast.Ident]types.Object{},
		Uses:  map[*ast.Ident]types.Object{},
		Types: map[ast.Expr]types.TypeAndValue{},
	}}
	// Nested selector with empty TypesInfo must not panic; returns "".
	sel := &ast.SelectorExpr{
		X:   &ast.SelectorExpr{X: &ast.Ident{Name: "sdk"}, Sel: &ast.Ident{Name: "resource"}},
		Sel: &ast.Ident{Name: "New"},
	}
	if got := packageOf(pass, sel); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := packageOf(pass, &ast.BasicLit{Kind: token.STRING, Value: `"x"`}); got != "" {
		t.Fatalf("non-ident: %q", got)
	}
	if fn := calleeFunc(pass, &ast.CallExpr{Fun: &ast.IndexExpr{X: &ast.Ident{Name: "f"}}}); fn != nil {
		t.Fatal("expected nil callee")
	}
}

func TestRunWithoutInspectorResult(t *testing.T) {
	got, err := run(&analysis.Pass{ResultOf: map[*analysis.Analyzer]any{}})
	if err != nil || got != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}
