// Package msgtrace finds Kafka produce without trace headers, including
// one-hop local wrappers around WriteMessages/SendMessage/Produce.
package msgtrace

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer reports headerless Kafka produces, following one hop into local
// wrappers. Ambiguous wrapper matches use medium confidence via the engine map.
var Analyzer = &analysis.Analyzer{
	Name:     "msgtrace",
	Doc:      "report Kafka produce without trace headers (one-hop wrappers)",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var produceMethods = map[string]bool{
	"WriteMessages": true,
	"SendMessage":   true,
	"SendMessages":  true,
	"Produce":       true,
	"ProduceSync":   true,
}

func run(pass *analysis.Pass) (any, error) { //nolint:gocyclo // one-hop wrapper detection has several AST branches
	inspRaw := pass.ResultOf[inspect.Analyzer]
	insp, ok := inspRaw.(*inspector.Inspector)
	if !ok {
		return nil, nil
	}

	// Local funcs that forward a Message-shaped arg into a produce method.
	wrappers := map[*types.Func]int{} // param index of the message

	insp.Nodes([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node, push bool) bool {
		if !push {
			return false
		}
		fd, ok := n.(*ast.FuncDecl)
		if !ok || fd.Body == nil || fd.Name == nil || fd.Type.Params == nil {
			return false
		}
		obj, _ := pass.TypesInfo.ObjectOf(fd.Name).(*types.Func)
		if obj == nil {
			return false
		}
		paramIdx := messageParamIndex(pass, fd)
		if paramIdx < 0 {
			return false
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isProduceCall(call) {
				return true
			}
			wrappers[obj] = paramIdx
			return true
		})
		return false
	})

	insp.Nodes([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool) bool {
		if !push {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return false
		}

		// Direct produce with inline message literal.
		if isProduceCall(call) {
			for _, arg := range call.Args {
				if lit, ok := arg.(*ast.CompositeLit); ok && isMessageLit(pass, lit) && !hasHeaders(lit) {
					pass.Reportf(lit.Pos(), "Message published with no Headers, so consumers start orphan traces")
				}
			}
			return false
		}

		// One-hop wrapper call site.
		fn := calleeFunc(pass, call)
		idx, ok := wrappers[fn]
		if !ok || idx >= len(call.Args) {
			return false
		}
		arg := call.Args[idx]
		lit, ok := arg.(*ast.CompositeLit)
		if !ok || !isMessageLit(pass, lit) || hasHeaders(lit) {
			return false
		}
		pass.Report(analysis.Diagnostic{
			Pos:     lit.Pos(),
			Message: "Message published via wrapper with no Headers, so consumers start orphan traces",
		})
		return false
	})
	return nil, nil
}

func isProduceCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && produceMethods[sel.Sel.Name]
}

func messageParamIndex(pass *analysis.Pass, fd *ast.FuncDecl) int {
	i := 0
	for _, field := range fd.Type.Params.List {
		n := len(field.Names)
		if n == 0 {
			n = 1
		}
		if isMessageType(pass, field.Type) {
			return i
		}
		i += n
	}
	return -1
}

func isMessageType(pass *analysis.Pass, expr ast.Expr) bool {
	t := pass.TypesInfo.TypeOf(expr)
	if t == nil {
		return false
	}
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	if !ok || n.Obj() == nil || n.Obj().Pkg() == nil {
		return false
	}
	name := n.Obj().Name()
	path := n.Obj().Pkg().Path()
	switch name {
	case "Message":
		return strings.Contains(path, "kafka-go") || strings.HasSuffix(path, "/kafka")
	case "ProducerMessage":
		return strings.Contains(path, "sarama")
	case "Record":
		return strings.Contains(path, "kgo") || strings.Contains(path, "franz-go")
	default:
		return false
	}
}

func isMessageLit(pass *analysis.Pass, lit *ast.CompositeLit) bool {
	if lit.Type == nil {
		return false
	}
	return isMessageType(pass, lit.Type)
}

func hasHeaders(lit *ast.CompositeLit) bool {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "Headers" {
			return true
		}
	}
	return false
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
