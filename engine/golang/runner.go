// Package golang is the Go deep tier: go/packages + go/analysis into findings.
package golang

import (
	"fmt"
	"go/token"
	"log/slog"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/Pieczasz/tarsier/analyzers/msgtrace"
	"github.com/Pieczasz/tarsier/analyzers/otelresource"
	"github.com/Pieczasz/tarsier/finding"

	"github.com/jjti/go-spancheck"
	"github.com/kisielk/errcheck/errcheck"
	"github.com/kkHAIKE/contextcheck"
	"github.com/sonatard/noctx"
	"go-simpler.org/sloglint"
)

// meta maps analyzer names to finding metadata.
type meta struct {
	rule       string
	confidence string
	severity   string
}

const (
	sevWarning = "warning"
	sevInfo    = "info"
	confHigh   = "high"
	confMedium = "medium"
)

var analyzerMeta = map[string]meta{
	"otelresource": {rule: "otel/sdk-missing-resource-attrs", confidence: confHigh, severity: sevWarning},
	"msgtrace":     {rule: "msgtrace/kafka-produce-no-inject", confidence: confMedium, severity: sevWarning},
	"spancheck":    {rule: "traces/error-path-not-recorded-on-span", confidence: confMedium, severity: sevWarning},
	"contextcheck": {rule: "httpctx/outbound-call-without-context", confidence: confMedium, severity: sevWarning},
	"noctx":        {rule: "httpctx/outbound-call-without-context", confidence: confHigh, severity: sevWarning},
	"sloglint":     {rule: "logs/unstructured-logging", confidence: confMedium, severity: sevInfo},
	"errcheck":     {rule: "errors/swallowed-on-critical-path", confidence: confMedium, severity: sevWarning},
}

// Runner loads a Go module and runs the deep-tier analyzers.
type Runner struct {
	// Analyzers overrides the default set; nil means Defaults().
	Analyzers []*analysis.Analyzer
}

// Defaults returns wrapped linters plus tarsier custom passes.
func Defaults() []*analysis.Analyzer {
	cfg := spancheck.NewDefaultConfig()
	cfg.EnabledChecks = []string{"end", "record-error", "set-status"}
	return []*analysis.Analyzer{
		otelresource.Analyzer,
		msgtrace.Analyzer,
		spancheck.NewAnalyzerWithConfig(cfg),
		contextcheck.NewAnalyzer(contextcheck.Configuration{DisableFact: true}),
		noctx.Analyzer,
		sloglint.New(nil),
		errcheck.Analyzer,
	}
}

// Scan type-checks packages under root and returns normalized findings.
// Root must be a Go module (or contain one). Non-Go trees return nil, nil.
func (r *Runner) Scan(root string) ([]finding.Finding, error) { //nolint:gocyclo // load + filter + per-analyzer loop

	analyzers := r.Analyzers
	if analyzers == nil {
		analyzers = Defaults()
	}
	if err := analysis.Validate(analyzers); err != nil {
		return nil, fmt.Errorf("go analyzers: %w", err)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule,
		Dir:   root,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("go/packages load: %w", err)
	}
	if len(pkgs) == 0 {
		return nil, nil
	}
	// No Go files at all (e.g. JS-only tree) — not an error.
	hasGo := false
	for _, p := range pkgs {
		if len(p.GoFiles) > 0 || len(p.CompiledGoFiles) > 0 {
			hasGo = true
			break
		}
	}
	if !hasGo {
		return nil, nil
	}

	initial := make([]*packages.Package, 0, len(pkgs))
	for _, p := range pkgs {
		if packageUnderRoot(p, root) && len(p.Errors) == 0 {
			initial = append(initial, p)
		}
	}
	if len(initial) == 0 {
		return nil, nil
	}

	var out []finding.Finding
	seen := map[string]bool{}
	for _, a := range analyzers {
		got := runAnalyzer(a, initial, root)
		for i := range got {
			if seen[got[i].Fingerprint] {
				continue
			}
			seen[got[i].Fingerprint] = true
			out = append(out, got[i])
		}
	}
	return out, nil
}

// runAnalyzer runs one analyzer. Some third-party analyzers panic on partial
// dependency graphs (ctrlflow); isolate so one bad wrapper cannot kill the scan.
func runAnalyzer(a *analysis.Analyzer, pkgs []*packages.Package, root string) (out []finding.Finding) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Warn("go analyzer panicked; skipping", "analyzer", a.Name, "detail", fmt.Sprint(rec))
		}
	}()
	graph, err := checker.Analyze([]*analysis.Analyzer{a}, pkgs, &checker.Options{Sequential: true})
	if err != nil {
		slog.Warn("go analyzer failed", "analyzer", a.Name, "detail", err.Error())
		return nil
	}
	m, ok := analyzerMeta[a.Name]
	if !ok {
		return nil
	}
	for act := range graph.All() {
		if act.Err != nil || act.Package == nil || !packageUnderRoot(act.Package, root) {
			continue
		}
		for _, d := range act.Diagnostics {
			out = append(out, diagnosticFinding(root, act.Package.Fset, a.Name, m, &d))
		}
	}
	return out
}

func packageUnderRoot(pkg *packages.Package, root string) bool {
	root, _ = filepath.Abs(root)
	for _, f := range pkg.GoFiles {
		abs, err := filepath.Abs(f)
		if err != nil {
			continue
		}
		if strings.HasPrefix(abs, root+string(filepath.Separator)) || abs == root {
			return true
		}
	}
	return false
}

func diagnosticFinding(root string, fset *token.FileSet, analyzer string, m meta, d *analysis.Diagnostic) finding.Finding {
	pos := fset.Position(d.Pos)
	rel := pos.Filename
	if r, err := filepath.Rel(root, pos.Filename); err == nil && !strings.HasPrefix(r, "..") {
		rel = r
	}
	rel = filepath.ToSlash(rel)
	conf := m.confidence
	// Wrapper follow-ups from our analyzers stay medium and never raise.
	if strings.Contains(d.Message, "wrapper") {
		conf = confMedium
	}
	f := finding.Finding{
		Rule:       m.rule,
		Language:   "go",
		Severity:   m.severity,
		Confidence: conf,
		Location: finding.Location{
			File:   rel,
			Line:   pos.Line,
			Symbol: truncate(d.Message, 120),
		},
		Message: d.Message,
		Note:    "Go deep tier (" + analyzer + "). Medium confidence never fails a --fail-on gate by default.",
		Evidence: finding.Evidence{Static: map[string]any{
			"analyzer": analyzer,
			"engine":   "go",
		}},
		Status: finding.StatusOpen,
	}
	f.Fill()
	return f
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
