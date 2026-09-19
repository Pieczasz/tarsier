package pattern

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/Pieczasz/tarsier/finding"
)

func TestScanInterpretsExitCodes(t *testing.T) {
	t.Parallel()

	oneMatch := `{"text":"\"user_id\"","range":{"start":{"line":4,"column":1}},"file":"m.go","language":"Go","ruleId":"r-go","severity":"warning","metadata":{"rule":"metrics/high-cardinality-label"}}`

	tests := []struct {
		name         string
		stdout       string
		stderr       string
		exitCode     int
		wantFindings int
		wantErr      string
	}{
		{
			name:         "exit 0 with no matches",
			exitCode:     0,
			wantFindings: 0,
		},
		{
			name:         "exit 0 with matches",
			stdout:       oneMatch,
			exitCode:     0,
			wantFindings: 1,
		},
		{
			name:         "exit 1 means matches were found",
			stdout:       oneMatch,
			exitCode:     1,
			wantFindings: 1,
		},
		{
			name:     "exit 1 without matches is a failure",
			stderr:   "Error: cannot parse glob",
			exitCode: 1,
			wantErr:  "cannot parse glob",
		},
		{
			name:     "exit 2 is a real failure and surfaces stderr",
			stderr:   "Error: cannot parse rule config",
			exitCode: 2,
			wantErr:  "cannot parse rule config",
		},
		{
			name:     "malformed output is reported, not silently dropped",
			stdout:   `{"text":"unterminated`,
			exitCode: 0,
			wantErr:  "decode ast-grep output",
		},
		{
			name:         "a malformed tail keeps the findings already decoded",
			stdout:       oneMatch + "\n" + `{"text":"unterminated`,
			exitCode:     0,
			wantFindings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := &Runner{Bin: fakeAstGrep(t, tt.stdout, tt.stderr, minVersion, tt.exitCode)}
			got, err := r.Scan(context.Background(), ".")

			switch {
			case tt.wantErr != "":
				if err == nil {
					t.Fatalf("want error containing %q, got %d findings and no error", tt.wantErr, len(got))
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tt.wantErr)
				}
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			case len(got) != tt.wantFindings:
				t.Fatalf("got %d findings, want %d", len(got), tt.wantFindings)
			}
		})
	}
}

func TestScanKeepsFindingsWhenPathsAreSkipped(t *testing.T) {
	t.Parallel()

	oneMatch := `{"text":"\"user_id\"","range":{"start":{"line":4}},"file":"good/m.go","language":"Go","ruleId":"r","severity":"warning","metadata":{"rule":"metrics/high-cardinality-label"}}`

	t.Run("a clean tree with a skipped path stays clean", func(t *testing.T) {
		t.Parallel()

		r := &Runner{Bin: fakeAstGrep(t, "", "ERROR: secret: Permission denied (os error 13)", minVersion, 0)}
		got, err := r.Scan(context.Background(), ".")
		if err != nil {
			t.Fatalf("an unreadable file must not fail a clean scan: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("got %d findings, want none", len(got))
		}
	})

	t.Run("a skipped path keeps the findings", func(t *testing.T) {
		t.Parallel()

		r := &Runner{Bin: fakeAstGrep(t, oneMatch, "ERROR: secret: Permission denied (os error 13)", minVersion, 0)}
		got, err := r.Scan(context.Background(), ".")
		if err != nil {
			t.Fatalf("a partially readable tree must still report what it found: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d findings, want the 1 from the readable files", len(got))
		}
	})
}

func TestScanRejectsAMissingTarget(t *testing.T) {
	t.Parallel()

	r := &Runner{Bin: fakeAstGrep(t, "", "", minVersion, 0)}
	_, err := r.Scan(context.Background(), filepath.Join(t.TempDir(), "no-such-dir"))
	if err == nil || !strings.Contains(err.Error(), "cannot scan") {
		t.Fatalf("want a clear missing-target error, got %v", err)
	}
}

func TestScanSortsFindingsDeterministically(t *testing.T) {
	t.Parallel()

	// ast-grep walks files in parallel, so emission order is not stable.
	stream := `{"text":"b","range":{"start":{"line":4}},"file":"z.go","language":"Go","ruleId":"r","severity":"warning","metadata":{"rule":"x"}}
{"text":"a","range":{"start":{"line":1}},"file":"a.go","language":"Go","ruleId":"r","severity":"warning","metadata":{"rule":"x"}}
{"text":"c","range":{"start":{"line":9}},"file":"a.go","language":"Go","ruleId":"r","severity":"warning","metadata":{"rule":"x"}}`

	r := &Runner{Bin: fakeAstGrep(t, stream, "", minVersion, 0)}
	got, err := r.Scan(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}

	want := []struct {
		file string
		line int
	}{{"a.go", 2}, {"a.go", 10}, {"z.go", 5}}
	if len(got) != len(want) {
		t.Fatalf("got %d findings, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].Location.File != w.file || got[i].Location.Line != w.line {
			t.Errorf("position %d = %s:%d, want %s:%d",
				i, got[i].Location.File, got[i].Location.Line, w.file, w.line)
		}
	}
}

func TestScanRejectsOldAstGrep(t *testing.T) {
	t.Parallel()

	r := &Runner{Bin: fakeAstGrep(t, "", "", "0.30.0", 0)}
	_, err := r.Scan(context.Background(), ".")
	if err == nil || !strings.Contains(err.Error(), "too old") {
		t.Fatalf("want a version error, got %v", err)
	}
}

func TestScanExplainsAMissingBinary(t *testing.T) {
	t.Parallel()

	r := &Runner{Bin: "ast-grep-does-not-exist"}
	_, err := r.Scan(context.Background(), "testdata")
	if err == nil {
		t.Fatal("want an error when the binary is absent")
	}
	if !strings.Contains(err.Error(), "not found on PATH") {
		t.Errorf("error should tell the user how to install it, got: %v", err)
	}
}

func TestScanFindsPlantedLabelsInEveryLanguage(t *testing.T) {
	t.Parallel()

	got, err := realRunner(t).Scan(context.Background(), "testdata")
	if err != nil {
		t.Fatal(err)
	}

	type key struct{ lang, label string }
	want := map[key]bool{
		{"go", `"user_id"`}:       false, // orders_total
		{"go", `"user_id"#2`}:     false, // exports_total, same label, same file
		{"typescript", `'email'`}: false,
		{"java", `"session_id"`}:  false,
	}
	for _, f := range got {
		k := key{f.Language, f.Location.Symbol}
		if _, expected := want[k]; !expected {
			t.Errorf("unexpected finding: %s %s at %s:%d", f.Language, f.Location.Symbol, f.Location.File, f.Location.Line)
			continue
		}
		want[k] = true
	}
	for k, found := range want {
		if !found {
			t.Errorf("missed %s label %s", k.lang, k.label)
		}
	}
}

func TestScanIsImmuneToExcludedNamesAboveTheRoot(t *testing.T) {
	t.Parallel()

	r := realRunner(t)
	dir := filepath.Join(t.TempDir(), "build", "app")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join("testdata", "metrics.go"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metrics.go"), src, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := r.Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("an excluded directory name above the scan root must not empty the scan")
	}
}

func TestScanReportsRealLineNumbers(t *testing.T) {
	t.Parallel()

	got, err := realRunner(t).Scan(context.Background(), "testdata")
	if err != nil {
		t.Fatal(err)
	}

	src, err := os.ReadFile(filepath.Join("testdata", "metrics.go"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(src), "\n")

	var checked int
	for _, f := range got {
		if f.Location.File != "metrics.go" {
			continue
		}
		if f.Location.Line < 1 || f.Location.Line > len(lines) {
			t.Fatalf("line %d out of range for metrics.go", f.Location.Line)
		}
		if !strings.Contains(lines[f.Location.Line-1], "user_id") {
			t.Errorf("line %d is %q, which does not contain the reported label", f.Location.Line, lines[f.Location.Line-1])
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no Go findings to check line numbers against")
	}
}

const cannedStream = `
{"text":"\"user_id\"","range":{"start":{"line":8,"column":10},"end":{"line":8,"column":19}},"file":"api/metrics.go","language":"Go","ruleId":"metrics-high-cardinality-label-go","severity":"warning","message":"Metric label \"user_id\" is unbounded","metaVariables":{"single":{"LABEL":{"text":"\"user_id\""}}},"metadata":{"rule":"metrics/high-cardinality-label","requires":"bounded_cardinality","expectation_layer":"2","confidence":"high","spec_ref":"instrumentation-score/MET-001"}}
{"text":"\"user_id\"","range":{"start":{"line":13,"column":10},"end":{"line":13,"column":19}},"file":"api/metrics.go","language":"Go","ruleId":"metrics-high-cardinality-label-go","severity":"warning","message":"Metric label \"user_id\" is unbounded","metaVariables":{"single":{"LABEL":{"text":"\"user_id\""}}},"metadata":{"rule":"metrics/high-cardinality-label","expectation_layer":2}}
`

func TestDecodeMapsMatchesToFindings(t *testing.T) {
	t.Parallel()

	got, err := decode(strings.NewReader(cannedStream), ".", ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("decoded %d findings, want 2", len(got))
	}

	first := got[0]
	if first.Location.Line != 9 {
		t.Errorf("line = %d, want 9: ast-grep is 0-based, findings are 1-based", first.Location.Line)
	}
	if first.Rule != "metrics/high-cardinality-label" {
		t.Errorf("rule = %q, want the logical id from metadata, not the per-language ast-grep id", first.Rule)
	}
	if first.Confidence != "high" {
		t.Errorf("confidence = %q, want it carried over from rule metadata", first.Confidence)
	}
	if first.Evidence.Static["requires"] != "bounded_cardinality" {
		t.Errorf("static evidence lost `requires`: %v", first.Evidence.Static)
	}
	if first.Language != "go" {
		t.Errorf("language = %q, want lowercase", first.Language)
	}

	if got[0].Fingerprint == got[1].Fingerprint {
		t.Error("repeated matches in one file need distinct fingerprints")
	}
	if got[0].Location.Symbol == got[1].Location.Symbol {
		t.Error("repeated matches need distinguishable symbols")
	}

	if got[1].Rule != "metrics/high-cardinality-label" {
		t.Errorf("second finding rule = %q; a numeric metadata value broke decoding", got[1].Rule)
	}
}

func TestDecodeTruncatesLongSymbolsButFingerprintsFullText(t *testing.T) {
	t.Parallel()

	full := "kafka.Message{\n\t\tTopic: \"orders\",\n\t\tValue: []byte(orderID),\n\t}"
	stream := `{"text":` + strconv.Quote(full) + `,"range":{"start":{"line":13}},"file":"api/publish.go","language":"Go","ruleId":"msgtrace-kafka-produce-no-inject-go","severity":"warning","metadata":{"rule":"msgtrace/kafka-produce-no-inject"}}`

	got, err := decode(strings.NewReader(stream), ".", ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d findings, want 1", len(got))
	}
	f := got[0]

	if strings.Contains(f.Location.Symbol, "\n") {
		t.Errorf("symbol carries a multi-line match into JSON output: %q", f.Location.Symbol)
	}
	if n := len([]rune(f.Location.Symbol)); n > maxSymbolLen+len("#99") {
		t.Errorf("symbol length %d exceeds the display budget", n)
	}
	if matched, _ := f.Evidence.Static["matched"].(string); matched != full {
		t.Errorf("full match text lost from evidence: %q", f.Evidence.Static["matched"])
	}
	want := finding.Fingerprint("msgtrace/kafka-produce-no-inject", "", "api/publish.go", full)
	if f.Fingerprint != want {
		t.Error("fingerprint follows display truncation; baselines would break every time truncation changes - it must hash the full match text")
	}
}

func TestVersionAtLeast(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  string
		want string
		ok   bool
	}{
		{name: "newer patch", got: "0.45.1", want: "0.45.0", ok: true},
		{name: "exact match", got: "0.45.0", want: "0.45.0", ok: true},
		{name: "older minor", got: "0.44.9", want: "0.45.0", ok: false},
		{name: "newer major", got: "1.0.0", want: "0.45.0", ok: true},
		{name: "missing patch component", got: "0.46", want: "0.45.0", ok: true},
		{name: "older patch", got: "0.45.1", want: "0.45.2", ok: false},
		{name: "newer patch", got: "0.45.3", want: "0.45.2", ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := versionAtLeast(tt.got, tt.want); got != tt.ok {
				t.Errorf("versionAtLeast(%q, %q) = %v, want %v", tt.got, tt.want, got, tt.ok)
			}
		})
	}
}

func TestExcludeGlobsCoverExactNames(t *testing.T) {
	t.Parallel()

	globs := excludeGlobs()
	patterns := map[string]bool{}
	for i := 0; i+1 < len(globs); i += 2 {
		if globs[i] != "--globs" {
			t.Fatalf("expected --globs at %d, got %q", i, globs[i])
		}
		patterns[globs[i+1]] = true
	}
	for _, want := range []string{
		"!**/vendor/**",
		"!**/_vendor/**",
		"!**/test/**",
		"!**/testmocks/**",
		"!**/testhelpers/**",
		"!**/node_modules/**",
		"!**/*_test.go",
		"!**/*Test.java",
	} {
		if !patterns[want] {
			t.Errorf("excludeGlobs missing %q", want)
		}
	}
}

func TestCompoundTokenPostfilter(t *testing.T) {
	t.Parallel()

	// Post-filter keeps only hyphen/underscore compounds. Exact directory
	// names (vendor, testmocks) and suffixes are glob-only and must not
	// match here - proving the dual path is narrowed, not duplicated.
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "compound test directory", path: "kubernetes-tests/lang_sdk/main.go", want: true},
		{name: "compound example directory", path: "sdk/go_example/main.go", want: true},
		{name: "exact acceptance is glob-only", path: "apps/login/acceptance/oidcrp/main.go", want: false},
		{name: "exact vendor is glob-only", path: "vendor/github.com/grafana/dskit/runtimeconfig/provider.go", want: false},
		{name: "exact testmocks is glob-only", path: "testmocks/src/main/java/org/apache/bookkeeper/client/PulsarMockBookKeeper.java", want: false},
		{name: "exact _vendor is glob-only", path: "_vendor/croniter/__init__.py", want: false},
		{name: "suffix _test.go is glob-only", path: "modules/frontend/queue/queue_test.go", want: false},
		{name: "exact test dir is glob-only", path: "src/test/java/org/apache/pulsar/BrokerTest.java", want: false},
		{name: "production go", path: "modules/frontend/v1/frontend.go", want: false},
		{name: "file merely starting with vendor", path: "internal/vendored.go", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := excluded(tt.path); got != tt.want {
				t.Errorf("excluded(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestDecodeDropsCompoundTokenPathsGlobsCannotExpress(t *testing.T) {
	t.Parallel()

	// !**/test/** does not match kubernetes-tests/; the post-filter must.
	stream := `{"text":"\"user_id\"","range":{"start":{"line":1}},"file":"kubernetes-tests/lang_sdk/main.go","language":"Go","ruleId":"r","severity":"warning","metadata":{"rule":"metrics/high-cardinality-label"}}
{"text":"\"user_id\"","range":{"start":{"line":2}},"file":"modules/frontend/v1/frontend.go","language":"Go","ruleId":"r","severity":"warning","metadata":{"rule":"metrics/high-cardinality-label"}}
`
	got, err := decode(strings.NewReader(stream), ".", ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("decoded %d findings, want 1 production path", len(got))
	}
	if got[0].Location.File != "modules/frontend/v1/frontend.go" {
		t.Errorf("kept %q, want the production path only", got[0].Location.File)
	}
}

func realRunner(t *testing.T) *Runner {
	t.Helper()
	if _, err := exec.LookPath(defaultBin); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("ast-grep must be on PATH in CI")
		}
		t.Skip("ast-grep not on PATH; `brew install ast-grep` to run driver tests")
	}
	config, err := filepath.Abs(filepath.Join("..", "..", "sgconfig.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return &Runner{Config: config}
}

func fakeAstGrep(t *testing.T, stdout, stderr, version string, exitCode int) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary is a POSIX shell script")
	}

	dir := t.TempDir()
	outFile := filepath.Join(dir, "stdout")
	errFile := filepath.Join(dir, "stderr")
	for path, content := range map[string]string{outFile: stdout, errFile: stderr} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "--version" ]; then echo "ast-grep %s"; exit 0; fi
cat %q
cat %q >&2
exit %d
`, version, outFile, errFile, exitCode)

	return writeExecScript(t, dir, "ast-grep", script)
}

func fakeAstGrepVersionOnly(t *testing.T, versionLine string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake binary is a POSIX shell script")
	}
	dir := t.TempDir()
	return writeExecScript(t, dir, "ast-grep", fmt.Sprintf("#!/bin/sh\necho %q\n", versionLine))
}

// writeExecScript writes a shell script via rename so exec never races a
// still-open writer (ETXTBSY on overlayfs).
func writeExecScript(t *testing.T, dir, name, script string) string {
	t.Helper()
	tmp := filepath.Join(dir, name+".tmp")
	bin := filepath.Join(dir, name)
	if err := os.WriteFile(tmp, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, bin); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestFingerprintsSurviveAnInsertionAbove(t *testing.T) {
	t.Parallel()

	line := func(n string) string {
		return `{"text":"\"user_id\"","range":{"start":{"line":` + n +
			`}},"file":"m.go","language":"Go","ruleId":"r","severity":"warning","metadata":{"rule":"metrics/high-cardinality-label"}}`
	}

	before, err := decode(strings.NewReader(line("10")+"\n"+line("20")), ".", ".")
	if err != nil {
		t.Fatal(err)
	}
	after, err := decode(strings.NewReader(line("5")+"\n"+line("10")+"\n"+line("20")), ".", ".")
	if err != nil {
		t.Fatal(err)
	}
	if before[0].Fingerprint != after[0].Fingerprint {
		t.Error("fingerprint changed after an unrelated match was added above")
	}
}

func TestTruncateBase(t *testing.T) {
	t.Parallel()

	if got := truncateBase("short"); got != "short" {
		t.Fatalf("got %q", got)
	}
	multi := "line one\nline two\nline three"
	if got := truncateBase(multi); strings.Contains(got, "\n") {
		t.Fatalf("want flattened, got %q", got)
	}
	long := strings.Repeat("字", maxSymbolLen+10)
	got := truncateBase(long)
	if n := len([]rune(got)); n != maxSymbolLen {
		t.Fatalf("len=%d want %d (%q)", n, maxSymbolLen, got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("want ellipsis, got %q", got)
	}
}

func TestRelPathEdges(t *testing.T) {
	t.Parallel()

	if got := relPath("api/metrics.go", "metrics.go"); got != "api/metrics.go" {
		t.Fatalf("basename under file root: %q", got)
	}
	if got := relPath("/repo", "/repo"); got != "/repo" {
		t.Fatalf("same path: %q", got)
	}
	if got := relPath("/repo", "/repo/a.go"); got != "a.go" {
		t.Fatalf("child: %q", got)
	}
}

func TestWaitErrBranches(t *testing.T) {
	t.Parallel()

	var stderr strings.Builder
	if err := waitErr(context.Background(), nil, &stderr, false); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitErr(ctx, context.Canceled, &stderr, false); err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("want canceled, got %v", err)
	}

	dctx, dcancel := context.WithTimeout(context.Background(), 0)
	<-dctx.Done()
	dcancel()
	if err := waitErr(dctx, context.DeadlineExceeded, &stderr, false); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("want timeout, got %v", err)
	}

	if err := waitErr(context.Background(), errors.New("boom"), &stderr, false); err == nil || !strings.Contains(err.Error(), "ast-grep failed") {
		t.Fatalf("want failed without stderr detail, got %v", err)
	}
}

func TestCheckVersionUnparseable(t *testing.T) {
	t.Parallel()

	bin := fakeAstGrepVersionOnly(t, "not-a-version")
	err := checkVersion(context.Background(), bin)
	if err == nil || !strings.Contains(err.Error(), "cannot parse") {
		t.Fatalf("got %v", err)
	}
}

func TestScanSingleFileTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "m.go")
	if err := os.WriteFile(file, []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := &Runner{Bin: fakeAstGrep(t, "", "", minVersion, 0)}
	got, err := r.Scan(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestScanMergesCardinalityExtras(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := []byte(`package shop;
class M {
  private static final String[] LABELS = { "user_id" };
  void f(){ Counter.build().labelNames(LABELS).register(); }
}
`)
	if err := os.WriteFile(filepath.Join(dir, "M.java"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	r := &Runner{Bin: fakeAstGrep(t, "", "", minVersion, 0)}
	got, err := r.Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("want cardinality extras from Java const labels")
	}
}

func TestScanSortsMergedExtras(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := []byte(`package shop;
class M {
  private static final String[] A = { "user_id" };
  private static final String[] B = { "email" };
  void f(){
    Counter.build().labelNames(A).register();
    Counter.build().labelNames(B).register();
  }
}
`)
	if err := os.WriteFile(filepath.Join(dir, "M.java"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	r := &Runner{Bin: fakeAstGrep(t, "", "", minVersion, 0)}
	got, err := r.Scan(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 2 {
		t.Fatalf("want >=2 extras for sort, got %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		a, b := got[i-1], got[i]
		if a.Location.File > b.Location.File {
			t.Fatalf("unsorted by file: %q > %q", a.Location.File, b.Location.File)
		}
		if a.Location.File == b.Location.File && a.Location.Line > b.Location.Line {
			t.Fatalf("unsorted by line: %d > %d", a.Location.Line, b.Location.Line)
		}
	}
}

func TestDecodeEmptyDirUsesRoot(t *testing.T) {
	t.Parallel()
	got, err := decode(strings.NewReader(cannedStream), "/scan-root", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("want findings")
	}
}
