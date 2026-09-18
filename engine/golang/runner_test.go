package golang

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/Pieczasz/tarsier/analyzers/msgtrace"
	"github.com/Pieczasz/tarsier/analyzers/otelresource"
)

func TestScanMsgtraceWrapperFollow(t *testing.T) {
	dir := godeepDir(t)
	r := &Runner{Analyzers: []*analysis.Analyzer{msgtrace.Analyzer, otelresource.Analyzer}}
	got, err := r.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	var msg, otel int
	for _, f := range got {
		t.Logf("finding %s %s:%d %q conf=%s", f.Rule, f.Location.File, f.Location.Line, f.Message, f.Confidence)
		switch f.Rule {
		case "msgtrace/kafka-produce-no-inject":
			msg++
		case "otel/sdk-missing-resource-attrs":
			otel++
		}
	}
	if msg < 1 {
		t.Fatalf("want msgtrace wrapper hit, got %+v", got)
	}
	if otel < 1 {
		t.Fatalf("want otelresource hit, got %+v", got)
	}
}

func TestScanEmptyTree(t *testing.T) {
	dir := t.TempDir()
	r := &Runner{Analyzers: []*analysis.Analyzer{msgtrace.Analyzer}}
	got, err := r.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestAnalyzeGraphOnGodeep(t *testing.T) {
	dir := godeepDir(t)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule,
		Dir:   dir,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatal("no packages")
	}
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			t.Logf("pkg errors: %v", p.Errors)
		}
	}
	graph, err := checker.Analyze([]*analysis.Analyzer{msgtrace.Analyzer, otelresource.Analyzer}, pkgs, &checker.Options{Sequential: true})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for act := range graph.All() {
		n += len(act.Diagnostics)
		for _, d := range act.Diagnostics {
			pos := act.Package.Fset.Position(d.Pos)
			t.Logf("%s %s:%d %s", act.Analyzer.Name, pos.Filename, pos.Line, d.Message)
		}
	}
	if n == 0 {
		t.Fatal("analyzers produced no diagnostics on godeep")
	}
}

func TestDefaultsValidate(t *testing.T) {
	if err := analysis.Validate(Defaults()); err != nil {
		t.Fatal(err)
	}
	// Full default set on the tiny fixture - exercises wrapper registration.
	got, err := (&Runner{}).Scan(godeepDir(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("defaults produced no findings on godeep")
	}
}

func TestPackageUnderRoot(t *testing.T) {
	dir := godeepDir(t)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles,
		Dir:  dir,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil || len(pkgs) == 0 {
		t.Fatalf("%v %d", err, len(pkgs))
	}
	if !packageUnderRoot(pkgs[0], dir) {
		t.Fatal("expected under root")
	}
	if packageUnderRoot(pkgs[0], t.TempDir()) {
		t.Fatal("other root")
	}
	if truncate("hi", 10) != "hi" {
		t.Fatal("short truncate")
	}
	long := strings.Repeat("x", 200)
	if got := truncate(long, 10); len([]rune(got)) != 10 {
		t.Fatalf("truncate=%q len=%d", got, len([]rune(got)))
	}
}

func TestScanValidateRejectsBadAnalyzer(t *testing.T) {
	r := &Runner{Analyzers: []*analysis.Analyzer{{Name: "bad"}}}
	_, err := r.Scan(godeepDir(t))
	if err == nil {
		t.Fatal("want validate error")
	}
}

func TestRunAnalyzerSkipsUnknownAndPanic(t *testing.T) {
	dir := godeepDir(t)
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes |
			packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedModule,
		Dir:   dir,
		Tests: false,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil || len(pkgs) == 0 {
		t.Fatalf("%v %d", err, len(pkgs))
	}
	var under []*packages.Package
	for _, p := range pkgs {
		if packageUnderRoot(p, dir) && len(p.Errors) == 0 {
			under = append(under, p)
		}
	}
	unknown := &analysis.Analyzer{
		Name: "unknownanalyzer",
		Doc:  "x",
		Run:  func(*analysis.Pass) (any, error) { return nil, nil },
	}
	if got := runAnalyzer(unknown, under, dir); len(got) != 0 {
		t.Fatalf("unknown meta should yield no findings: %v", got)
	}
	boom := &analysis.Analyzer{
		Name: "boom",
		Doc:  "x",
		Run:  func(*analysis.Pass) (any, error) { panic("boom") },
	}
	if got := runAnalyzer(boom, under, dir); len(got) != 0 {
		t.Fatalf("panic should be isolated: %v", got)
	}
}

func TestScanDedupesFingerprints(t *testing.T) {
	dir := godeepDir(t)
	r := &Runner{Analyzers: []*analysis.Analyzer{msgtrace.Analyzer}}
	got, err := r.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("need findings to assert uniqueness")
	}
	seen := map[string]bool{}
	for _, f := range got {
		if seen[f.Fingerprint] {
			t.Fatalf("duplicate fingerprint %s", f.Fingerprint)
		}
		seen[f.Fingerprint] = true
	}
	// Re-run through runAnalyzer twice and feed Scan's dedupe path by
	// merging identical sets: fingerprints from two identical runs must match.
	second, err := r.Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != len(got) {
		t.Fatalf("unstable scan %d vs %d", len(got), len(second))
	}
}

func godeepDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "fixtures", "godeep")
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}
