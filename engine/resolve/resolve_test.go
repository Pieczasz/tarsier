package resolve

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeGoImportsAndSliceConsts(t *testing.T) {
	t.Parallel()
	src := []byte(`package p
import (
	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel/attribute"
)
const label = "user_id"
var LABELS = []string{"session_id", "status"}
`)
	f := Analyze("x.go", src)
	if f.PackageOf("kafkago") != "github.com/segmentio/kafka-go" {
		t.Fatalf("alias: got %q", f.PackageOf("kafkago"))
	}
	if f.PackageOf("attribute") != "go.opentelemetry.io/otel/attribute" {
		t.Fatalf("attribute import: got %q", f.PackageOf("attribute"))
	}
	if !f.HasImport("otel/attribute") {
		t.Fatal("HasImport missed attribute")
	}
	if got := f.Strings["label"]; len(got) != 1 || got[0].Value != "user_id" {
		t.Fatalf("const label: %+v", got)
	}
	if got := f.Strings["LABELS"]; len(got) != 2 || got[0].Value != "session_id" {
		t.Fatalf("LABELS: %+v", got)
	}
}

func TestAnalyzeJavaConstArray(t *testing.T) {
	t.Parallel()
	src := []byte(`package shop.worker;
import io.prometheus.client.Counter;
public class Metrics {
  private static final String[] LABELS = {
    "user_id",
    "status",
  };
  public void registerIndirect() {
    Counter.build().labelNames(LABELS).register();
  }
}
`)
	f := Analyze("Metrics.java", src)
	if !f.HasImport("prometheus") {
		t.Fatalf("imports: %+v", f.Imports)
	}
	lits := f.Strings["LABELS"]
	if len(lits) != 2 {
		t.Fatalf("LABELS lits: %+v", lits)
	}
	if lits[0].Value != "user_id" || lits[0].Line != 5 {
		t.Fatalf("first lit: %+v", lits[0])
	}
}

func TestAnalyzePythonExceptionParam(t *testing.T) {
	t.Parallel()
	src := []byte(`
from prometheus_client import Counter
def record_failure(exc: Exception, status: str, request) -> None:
    lookups.labels(str(exc)).inc()
    lookups.labels(str(status)).inc()
`)
	f := Analyze("m.py", src)
	if f.Params["exc"] != "Exception" {
		t.Fatalf("params: %+v", f.Params)
	}
	if f.PackageOf("Counter") == "" {
		t.Fatalf("imports: %+v", f.Imports)
	}
}

func TestAnalyzeTSImports(t *testing.T) {
	t.Parallel()
	src := []byte(`import { metrics } from '@opentelemetry/api';
import kafka from 'kafkajs';
const LABELS = ['user_id', 'status'];
`)
	f := Analyze("m.ts", src)
	if !f.HasImport("@opentelemetry/api") {
		t.Fatalf("imports: %+v", f.Imports)
	}
	if f.PackageOf("kafka") != "kafkajs" {
		t.Fatalf("kafka: %q", f.PackageOf("kafka"))
	}
	if got := f.Strings["LABELS"]; len(got) != 2 || got[0].Value != "user_id" {
		t.Fatalf("LABELS: %+v", got)
	}
}

func TestCardinalityExtrasJavaAndPython(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	java := []byte(`package shop.worker;
import io.prometheus.client.Counter;
public class Metrics {
  private static final String[] LABELS = {
    "user_id",
  };
  public void registerIndirect() {
    Counter.build().labelNames(LABELS).register();
  }
}
`)
	py := []byte(`
def record_failure(exc: Exception, status: str) -> None:
    lookups.labels(str(exc)).inc()
    lookups.labels(str(status)).inc()
    lookups.labels(str(message.kind)).inc()
`)
	if err := write(dir, "Metrics.java", java); err != nil {
		t.Fatal(err)
	}
	if err := write(dir, "m.py", py); err != nil {
		t.Fatal(err)
	}
	got, err := CardinalityExtras(dir)
	if err != nil {
		t.Fatal(err)
	}
	var javaN, pyN int
	for _, f := range got {
		switch f.Rule {
		case ruleHighCard:
			javaN++
			if f.Location.Line != 5 {
				t.Fatalf("java line = %d, want 5", f.Location.Line)
			}
		case ruleUnbounded:
			pyN++
			if f.Location.Symbol != "str(exc)" {
				t.Fatalf("python symbol = %q", f.Location.Symbol)
			}
		}
	}
	if javaN != 1 || pyN != 1 {
		t.Fatalf("got java=%d python=%d findings: %+v", javaN, pyN, got)
	}
}

func TestCardinalityExtrasSingleFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "Metrics.java")
	src := []byte(`package p;
import io.prometheus.client.Counter;
class M {
  private static final String[] LABELS = { "email" };
  private static final String[] UNUSED = { "user_id" };
  void f() { Counter.build().labelNames(LABELS).register(); }
}
`)
	if err := os.WriteFile(path, src, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := CardinalityExtras(path)
	if err != nil || len(got) != 1 {
		t.Fatalf("got=%v err=%v", got, err)
	}
	if got[0].Location.Symbol != "email" {
		t.Fatalf("symbol=%q", got[0].Location.Symbol)
	}
}

func TestAnalyzeGoVersionedAndBlank(t *testing.T) {
	t.Parallel()
	src := []byte(`package p
import (
	_ "github.com/lib/pq"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)
const x = 1
var y = []int{1}
`)
	f := Analyze("x.go", src)
	if _, ok := f.Imports["_"]; ok {
		t.Fatal("blank import should be skipped")
	}
	if f.PackageOf("semconv") != "go.opentelemetry.io/otel/semconv/v1.24.0" {
		t.Fatalf("semconv: %q", f.PackageOf("semconv"))
	}
	if defaultImportName("go.opentelemetry.io/otel/semconv/v1.24.0") != "semconv" {
		t.Fatal(defaultImportName("go.opentelemetry.io/otel/semconv/v1.24.0"))
	}
}

func TestAnalyzePythonImportVariants(t *testing.T) {
	t.Parallel()
	src := []byte(`
import prometheus_client as pc
from opentelemetry import metrics, trace
import os
def f(exc: BaseException, other: ValueError) -> None:
    pass
LABELS = ["user_id"]
`)
	f := Analyze("m.py", src)
	if f.PackageOf("pc") != "prometheus_client" {
		t.Fatalf("pc: %q imports=%v", f.PackageOf("pc"), f.Imports)
	}
	if f.PackageOf("metrics") == "" || f.PackageOf("os") != "os" {
		t.Fatalf("imports: %+v", f.Imports)
	}
	if f.Params["exc"] != "BaseException" {
		t.Fatalf("params: %+v", f.Params)
	}
	if !isExceptionType("BaseException") || isExceptionType("ValueError") {
		t.Fatal("exception type gate")
	}
}

func TestNilFileHelpers(t *testing.T) {
	t.Parallel()
	var f *File
	if f.PackageOf("x") != "" || f.HasImport("x") {
		t.Fatal("nil file")
	}
	if LangOf("a.rs") != "" || LangOf("a.tsx") != "typescript" || LangOf("a.py") != "python" {
		t.Fatal(LangOf("a.tsx"), LangOf("a.py"))
	}
}

func TestCardinalityExtrasSkipsVendor(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bad := []byte(`class M { private static final String[] LABELS = { "user_id" }; void f(){ x.labelNames(LABELS); } }`)
	for _, skip := range []string{"vendor", "node_modules", ".git"} {
		sdir := filepath.Join(dir, skip)
		if err := os.MkdirAll(sdir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(sdir, "M.java"), bad, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Control file outside skip dirs must still fire.
	good := []byte(`package p;
class M {
  private static final String[] LABELS = { "user_id" };
  void f(){ x.labelNames(LABELS); }
}
`)
	if err := os.WriteFile(filepath.Join(dir, "Ok.java"), good, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := CardinalityExtras(dir)
	if err != nil || len(got) != 1 {
		t.Fatalf("skip dirs leaked or control missed: %v %v", got, err)
	}
	if got[0].Fingerprint == "" {
		t.Fatal("Fill must set fingerprint")
	}
}

func TestCardinalityExtrasJavaTagsAndPythonComment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	java := []byte(`package p;
class M {
  private static final String[] LABELS = { "email" };
  void f(){ meter.tags(LABELS); }
}
`)
	py := []byte(`
def record_failure(exc: Exception) -> None:
    lookups.labels(str(exc)).inc()  # noise
`)
	if err := write(dir, "M.java", java); err != nil {
		t.Fatal(err)
	}
	if err := write(dir, "m.py", py); err != nil {
		t.Fatal(err)
	}
	got, err := CardinalityExtras(dir)
	if err != nil {
		t.Fatal(err)
	}
	var javaN, pyN int
	for _, f := range got {
		if f.Fingerprint == "" {
			t.Fatal("missing fingerprint after Fill")
		}
		switch f.Rule {
		case ruleHighCard:
			javaN++
			if f.Location.Symbol != "email" {
				t.Fatalf("java symbol=%q", f.Location.Symbol)
			}
		case ruleUnbounded:
			pyN++
			ev, _ := f.Evidence.Static["matched"].(string)
			if strings.Contains(ev, "#") {
				t.Fatalf("comment not stripped: %q", ev)
			}
			if f.Location.Line != 3 {
				t.Fatalf("python line=%d", f.Location.Line)
			}
		}
	}
	if javaN != 1 || pyN != 1 {
		t.Fatalf("java=%d py=%d got=%+v", javaN, pyN, got)
	}
}

func TestHasImportEmptyAndBoundedJavaLit(t *testing.T) {
	t.Parallel()
	f := Analyze("x.go", []byte("package p\n"))
	if f.HasImport("anything") {
		t.Fatal("empty imports")
	}
	dir := t.TempDir()
	src := []byte(`package p;
class M {
  private static final String[] LABELS = { "status" };
  void f(){ x.labelNames(LABELS); }
}
`)
	if err := write(dir, "M.java", src); err != nil {
		t.Fatal(err)
	}
	got, err := CardinalityExtras(dir)
	if err != nil || len(got) != 0 {
		t.Fatalf("bounded label should not fire: %v %v", got, err)
	}
}

func TestDefaultImportNameVersionEdges(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"go.opentelemetry.io/otel/semconv/v1.24.0": "semconv",
		"example.com/mod/v0":                       "mod",
		"example.com/mod/v9":                       "mod",
		"v1":                                       "v1", // single segment: keep as-is
		"example.com/mod/vX":                       "vX", // not a version digit
		"example.com/mod/vault":                    "vault",
	}
	for path, want := range cases {
		if got := defaultImportName(path); got != want {
			t.Fatalf("%s: got %q want %q", path, got, want)
		}
	}
}

func TestAnalyzeGoDotImportAndBlankName(t *testing.T) {
	t.Parallel()
	src := []byte(`package p
import (
	. "fmt"
	_ "github.com/lib/pq"
)
var _ = []string{"user_id"}
const label = "email"
`)
	f := Analyze("x.go", src)
	if _, ok := f.Imports["fmt"]; ok {
		t.Fatalf("dot import should be skipped: %+v", f.Imports)
	}
	if _, ok := f.Imports["_"]; ok {
		t.Fatal("blank import kept")
	}
	if len(f.Strings["_"]) != 0 {
		t.Fatalf("blank var kept: %+v", f.Strings)
	}
	if f.Strings["label"][0].Value != "email" {
		t.Fatalf("%+v", f.Strings)
	}
}

func TestPythonCommentOnlyMatchAndStarImport(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// '#' at index 0 distinguishes >=0 vs >0 strip; matched evidence must not keep '#'.
	py := []byte(`
def record_failure(exc: Exception) -> None:
#lookups.labels(str(exc)).inc()
    pass
`)
	if err := write(dir, "m.py", py); err != nil {
		t.Fatal(err)
	}
	got, err := CardinalityExtras(dir)
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	ev, _ := got[0].Evidence.Static["matched"].(string)
	if strings.HasPrefix(ev, "#") || strings.Contains(ev, "#") {
		t.Fatalf("comment strip failed: %q", ev)
	}
	if got[0].Fingerprint == "" {
		t.Fatal("Fill required")
	}

	f := Analyze("m.py", []byte(`
from pkg import *
from pkg import Counter, 
import pkg.sub as alias
`))
	if f.PackageOf("*") != "" {
		t.Fatal("star import should not bind")
	}
	if f.PackageOf("alias") != "pkg.sub" {
		t.Fatalf("%+v", f.Imports)
	}
}

func TestTSJavaLineNumbersKillArithmetic(t *testing.T) {
	t.Parallel()
	ts := Analyze("m.ts", []byte("const LABELS = [\n  'user_id',\n  'session_id',\n]\n"))
	if len(ts.Strings["LABELS"]) != 2 {
		t.Fatalf("%+v", ts.Strings)
	}
	if ts.Strings["LABELS"][0].Line != 2 || ts.Strings["LABELS"][1].Line != 3 {
		t.Fatalf("ts lines: %+v", ts.Strings["LABELS"])
	}
	one := Analyze("m.ts", []byte(`const label = 'user_id';`))
	if one.Strings["label"][0].Line != 1 {
		t.Fatalf("%+v", one.Strings)
	}
	java := Analyze("M.java", []byte("class M {\n  private static final String[] LABELS = {\n    \"email\",\n  };\n}\n"))
	if java.Strings["LABELS"][0].Line != 3 {
		t.Fatalf("%+v", java.Strings)
	}
	py := Analyze("m.py", []byte("LABELS = ['user_id']\n"))
	if py.Strings["LABELS"][0].Line != 1 || py.Strings["LABELS"][0].Value != "user_id" {
		t.Fatalf("%+v", py.Strings)
	}
}

func TestNestedRelPathToSlash(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sub := filepath.Join(dir, "shop", "worker")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	src := []byte(`package p;
class M {
  private static final String[] LABELS = { "user_id" };
  void f(){ x.labelNames(LABELS); }
}
`)
	if err := os.WriteFile(filepath.Join(sub, "M.java"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := CardinalityExtras(dir)
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	if got[0].Location.File != "shop/worker/M.java" {
		t.Fatalf("ToSlash rel path: %q", got[0].Location.File)
	}
}

func TestPythonImportDottedAndSpaced(t *testing.T) {
	t.Parallel()
	f := Analyze("m.py", []byte("import pkg.sub\nfrom pkg import   Counter\n"))
	if f.PackageOf("sub") != "pkg.sub" {
		t.Fatalf("dotted import name: %+v", f.Imports)
	}
	if f.PackageOf("Counter") != "pkg.Counter" {
		t.Fatalf("spaced from-import: %+v", f.Imports)
	}
}

func TestDefaultImportNameSingleCharV(t *testing.T) {
	t.Parallel()
	// len(name)==1 must not be treated as a version segment (would panic on name[1]).
	if got := defaultImportName("example.com/mod/v"); got != "v" {
		t.Fatalf("got %q", got)
	}
}

func TestAnalyzeGoUnparsableAndNonString(t *testing.T) {
	t.Parallel()
	f := Analyze("x.go", []byte("package p\nconst x = !\n"))
	if len(f.Imports) != 0 || len(f.Strings) != 0 {
		t.Fatalf("unparsable should yield empty facts: %+v", f)
	}
	f = Analyze("x.go", []byte("package p\nconst n = 42\nvar s = \"ok\"\n"))
	if len(f.Strings["n"]) != 0 {
		t.Fatalf("non-string const: %+v", f.Strings)
	}
	if f.Strings["s"][0].Value != "ok" {
		t.Fatalf("%+v", f.Strings)
	}
}

func TestTSSideEffectImport(t *testing.T) {
	t.Parallel()
	f := Analyze("m.ts", []byte(`import '@opentelemetry/auto-instrumentations-node';
export const x = 'user_id';
`))
	if !f.HasImport("auto-instrumentations-node") {
		t.Fatalf("%+v", f.Imports)
	}
}

func TestUnboundedLabel(t *testing.T) {
	t.Parallel()
	if !UnboundedLabel(`"user_id"`) || !UnboundedLabel("session_id") {
		t.Fatal("expected unbounded")
	}
	if UnboundedLabel("status") || UnboundedLabel("tenant_id") {
		t.Fatal("expected bounded")
	}
}

func TestAnalyzeGoMultiName(t *testing.T) {
	t.Parallel()
	src := []byte(`package p
var a, b = []string{"user_id"}, []string{"status"}
const (
	c, d = "email", "region"
)
`)
	f := Analyze("x.go", src)
	if len(f.Strings["a"]) != 1 || f.Strings["a"][0].Value != "user_id" {
		t.Fatalf("a: %+v", f.Strings["a"])
	}
	if f.Strings["c"][0].Value != "email" || f.Strings["d"][0].Value != "region" {
		t.Fatalf("%+v", f.Strings)
	}
}

func TestPythonFromImportAliasAndStar(t *testing.T) {
	t.Parallel()
	src := []byte(`
from prometheus_client import Counter as C, *
from x import *
`)
	f := Analyze("m.py", src)
	if f.PackageOf("C") != "prometheus_client.Counter" {
		t.Fatalf("%+v", f.Imports)
	}
}

func TestJavaUnclosedAndExtrasErrors(t *testing.T) {
	t.Parallel()
	f := Analyze("M.java", []byte(`class M { private static final String[] LABELS = { "user_id" `))
	if len(f.Strings["LABELS"]) != 0 {
		t.Fatalf("unclosed: %+v", f.Strings)
	}
	if _, err := CardinalityExtras("/no/such/path-xyz"); err == nil {
		t.Fatal("want error")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := CardinalityExtras(dir)
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestTSMultilineArrayUnclosed(t *testing.T) {
	t.Parallel()
	f := Analyze("m.ts", []byte("const LABELS = [\n  'user_id',\n"))
	if len(f.Strings["LABELS"]) != 0 {
		t.Fatalf("%+v", f.Strings)
	}
}

func TestCardinalityExtrasSkipsSymlinkEscape(t *testing.T) {
	t.Parallel()
	outside := t.TempDir()
	leak := []byte(`package p;
class Leak {
  private static final String[] LABELS = { "user_id" };
  void f(){ x.labelNames(LABELS); }
}
`)
	if err := os.WriteFile(filepath.Join(outside, "Leak.java"), leak, 0o600); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	link := filepath.Join(dir, "Escape.java")
	if err := os.Symlink(filepath.Join(outside, "Leak.java"), link); err != nil {
		t.Skipf("symlink not available: %v", err)
	}
	got, err := CardinalityExtras(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("symlink escape leaked findings: %+v", got)
	}
}

func write(dir, name string, body []byte) error {
	return os.WriteFile(filepath.Join(dir, name), body, 0o600)
}

func TestPathInsideRootEdges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !pathInsideRoot(abs, filepath.Join(dir, "a.go")) {
		// file need not exist for Abs; EvalSymlinks may fail - create it
		if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if !pathInsideRoot(abs, filepath.Join(dir, "a.go")) {
			t.Fatal("expected inside")
		}
	}
	if pathInsideRoot(abs, filepath.Join(dir, "missing-dangling-xyz")) {
		t.Fatal("dangling path must be rejected")
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "b.go"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if pathInsideRoot(abs, filepath.Join(outside, "b.go")) {
		t.Fatal("outside path must be rejected")
	}
}

func TestExtrasForFileNonJavaPython(t *testing.T) {
	t.Parallel()
	got, err := extrasForFile("x.go", "x.go", []byte("package p\n"))
	if err != nil || len(got) != 0 {
		t.Fatalf("%v %v", got, err)
	}
}

func TestVisitCardinalityPathWalkError(t *testing.T) {
	t.Parallel()
	_, skip, err := visitCardinalityPath("/tmp", "/tmp", "/tmp/x", nil, errors.New("walk boom"))
	if !skip || err == nil {
		t.Fatalf("want skip+err, got skip=%v err=%v", skip, err)
	}
}
