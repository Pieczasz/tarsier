package msgtrace

import (
	"go/ast"
	"go/types"
	"testing"

	"golang.org/x/tools/go/analysis"
)

func TestCalleeFuncNonFuncAndHasHeadersSkip(t *testing.T) {
	pass := &analysis.Pass{TypesInfo: &types.Info{
		Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{},
	}}
	if fn := calleeFunc(pass, &ast.CallExpr{Fun: &ast.IndexExpr{X: &ast.Ident{Name: "f"}}}); fn != nil {
		t.Fatal("expected nil callee")
	}
	lit := &ast.CompositeLit{
		Elts: []ast.Expr{
			&ast.BasicLit{Value: `"t"`},
			&ast.KeyValueExpr{Key: &ast.Ident{Name: "Headers"}, Value: &ast.Ident{Name: "h"}},
		},
	}
	if !hasHeaders(lit) {
		t.Fatal("Headers key should win after skipping unkeyed elt")
	}
	if isMessageLit(pass, &ast.CompositeLit{}) {
		t.Fatal("nil type is not a message lit")
	}
}
