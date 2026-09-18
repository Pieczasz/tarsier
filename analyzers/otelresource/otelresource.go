// Package otelresource flags OTel resource construction without service.name,
// including one-hop local wrapper constructors.
package otelresource

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer reports resource.New (and thin wrappers around it) that never set
// service.name / WithFromEnv.
var Analyzer = &analysis.Analyzer{
	Name:     "otelresource",
	Doc:      "report OTel resource.New without service.name",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) { //nolint:gocyclo // wrapper + direct resource.New paths

	inspRaw := pass.ResultOf[inspect.Analyzer]
	insp, ok := inspRaw.(*inspector.Inspector)
	if !ok {
		return nil, nil
	}
	wrappers := map[*types.Func]bool{}

	// Pass 1: find local funcs whose body calls resource.New without attrs.
	insp.Nodes([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node, push bool) bool {
		if !push {
			return false
		}
		fd, ok := n.(*ast.FuncDecl)
		if !ok || fd.Body == nil || fd.Name == nil {
			return false
		}
		obj, _ := pass.TypesInfo.ObjectOf(fd.Name).(*types.Func)
		if obj == nil {
			return false
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isResourceNew(pass, call) {
				return true
			}
			if !hasServiceName(call) {
				wrappers[obj] = true
				pass.Reportf(call.Pos(), "OTel resource is created without service.name")
			}
			return true
		})
		return false
	})

	// Pass 2: call sites of those wrappers (one hop) - medium confidence.
	insp.Nodes([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool) bool {
		if !push {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return false
		}
		fn := calleeFunc(pass, call)
		if fn == nil || !wrappers[fn] {
			return false
		}
		// Skip the inner resource.New; pass 1 already reported it.
		if isResourceNew(pass, call) {
			return false
		}
		pass.Report(analysis.Diagnostic{
			Pos:     call.Pos(),
			Message: "wrapper constructs an OTel resource without service.name",
		})
		return false
	})
	return nil, nil
}

func isResourceNew(pass *analysis.Pass, call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "New" {
		return false
	}
	pkg := packageOf(pass, sel.X)
	return pkg != "" && strings.HasSuffix(pkg, "go.opentelemetry.io/otel/sdk/resource")
}

func packageOf(pass *analysis.Pass, expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		if obj := pass.TypesInfo.ObjectOf(x); obj != nil {
			if pn, ok := obj.(*types.PkgName); ok && pn.Imported() != nil {
				return pn.Imported().Path()
			}
		}
	case *ast.SelectorExpr:
		return packageOf(pass, x.X)
	}
	return ""
}

func hasServiceName(call *ast.CallExpr) bool {
	var b strings.Builder
	ast.Inspect(call, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.Ident:
			b.WriteString(t.Name)
			b.WriteByte(' ')
		case *ast.BasicLit:
			b.WriteString(t.Value)
			b.WriteByte(' ')
		}
		return true
	})
	s := b.String()
	return strings.Contains(s, "ServiceName") ||
		strings.Contains(s, "WithFromEnv") ||
		strings.Contains(s, "OTEL_SERVICE_NAME") ||
		strings.Contains(s, "service.name")
}

func calleeFunc(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		obj, _ := pass.TypesInfo.ObjectOf(fun).(*types.Func)
		return obj
	case *ast.SelectorExpr:
		obj, _ := pass.TypesInfo.ObjectOf(fun.Sel).(*types.Func)
		return obj
	}
	return nil
}
