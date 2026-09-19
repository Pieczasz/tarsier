package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Pieczasz/tarsier/finding"
)

func TestRender(t *testing.T) {
	t.Parallel()

	one := []finding.Finding{{
		Fingerprint: "abc123",
		Rule:        "metrics/high-cardinality-label",
		Severity:    "warning",
		Location:    finding.Location{File: "api/metrics.go", Line: 42},
		Message:     "Metric label \"user_id\" is unbounded",
	}}

	withNote := []finding.Finding{{
		Severity: "warning",
		Location: finding.Location{File: "a.go", Line: 1},
		Message:  "unbounded",
		Note:     "Drop the label or bucket it.",
	}}

	tests := []struct {
		name     string
		findings []finding.Finding
		format   string
		want     []string
		notWant  []string
	}{
		{
			name:     "text names the file, line and message",
			findings: one,
			format:   "text",
			want:     []string{"api/metrics.go:42: warning: Metric label", "1 findings"},
			notWant:  []string{"\n    "},
		},
		{
			name:     "text includes the remediation note",
			findings: withNote,
			format:   "text",
			want:     []string{"    Drop the label or bucket it."},
		},
		{
			name:   "text still reports a clean scan",
			format: "text",
			want:   []string{"0 findings"},
		},
		{
			name:     "json carries the fingerprint",
			findings: one,
			format:   "json",
			want:     []string{`"fingerprint": "abc123"`, `"rule": "metrics/high-cardinality-label"`},
		},
		{
			name:     "html is a self-contained report",
			findings: one,
			format:   "html",
			want:     []string{"<!DOCTYPE html>", "api/metrics.go:42", "tarsier scan"},
			notWant:  []string{"https://"},
		},
		{
			name:    "json emits an empty array, never null",
			format:  "json",
			want:    []string{"[]"},
			notWant: []string{"null"},
		},
		{
			name: "text marks suppressed findings",
			findings: []finding.Finding{{
				Severity: "warning",
				Location: finding.Location{File: "a.go", Line: 3},
				Message:  "orphan trace",
				Status:   finding.StatusSuppressed,
			}},
			format: "text",
			want:   []string{"a.go:3: warning: orphan trace [suppressed]", "1 findings"},
		},
		{
			name: "json preserves suppressed status",
			findings: []finding.Finding{{
				Severity: "warning",
				Location: finding.Location{File: "a.go", Line: 3},
				Message:  "orphan trace",
				Status:   finding.StatusSuppressed,
			}},
			format: "json",
			want:   []string{`"status": "suppressed"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			if err := render(&out, tt.findings, tt.format, "."); err != nil {
				t.Fatal(err)
			}
			for _, want := range tt.want {
				if !strings.Contains(out.String(), want) {
					t.Errorf("output missing %q:\n%s", want, out.String())
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(out.String(), notWant) {
					t.Errorf("output should not contain %q:\n%s", notWant, out.String())
				}
			}
		})
	}
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errors.New("write boom") }

func TestRenderTextWriteErrors(t *testing.T) {
	t.Parallel()
	findings := []finding.Finding{{
		Severity: "warning",
		Location: finding.Location{File: "a.go", Line: 1},
		Message:  "m",
		Note:     "note line",
	}}
	if err := render(errWriter{}, findings, "text", "."); err == nil {
		t.Fatal("want write error on finding line")
	}
	// Fail on note line: first Fprintf succeeds once, second fails.
	w := &failAfter{n: 1}
	if err := render(w, findings, "text", "."); err == nil {
		t.Fatal("want write error on note line")
	}
}

type failAfter struct {
	n, i int
}

func (f *failAfter) Write(p []byte) (int, error) {
	f.i++
	if f.i > f.n {
		return 0, errors.New("write boom")
	}
	return len(p), nil
}

func TestNormalizeEngineEmpty(t *testing.T) {
	t.Parallel()
	got, err := normalizeEngine("")
	if err != nil || got != enginePattern {
		t.Fatalf("got %q err=%v", got, err)
	}
}

func TestEnsurePatternRulesSkipsGoEngine(t *testing.T) {
	t.Parallel()
	cfg := ""
	cleanup, err := ensurePatternRules(engineGo, &cfg)
	if err != nil || cleanup != nil || cfg != "" {
		t.Fatalf("go engine must skip materialize: cfg=%q cleanup=%v err=%v", cfg, cleanup != nil, err)
	}
	cfg = "already.yml"
	cleanup, err = ensurePatternRules(enginePattern, &cfg)
	if err != nil || cleanup != nil || cfg != "already.yml" {
		t.Fatalf("preset rules must skip: cfg=%q cleanup=%v err=%v", cfg, cleanup != nil, err)
	}
}

func TestSourcePath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(sub, "a.go")
	if err := os.WriteFile(file, []byte("package p\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if got := sourcePath(dir, filepath.Join("sub", "a.go")); got != file {
		t.Errorf("dir root joins: got %q want %q", got, file)
	}
	if got := sourcePath(file, file); got != file {
		t.Errorf("single-file root returns the input: got %q want %q", got, file)
	}
	if got := sourcePath(file, "a.go"); got != file {
		t.Errorf("single-file root + basename: got %q want %q", got, file)
	}
	if got := sourcePath(file, "other.go"); got != filepath.Join(sub, "other.go") {
		t.Errorf("single-file root + sibling: got %q", got)
	}
	if got := sourcePath("/repo", "/abs/a.go"); got != "/abs/a.go" {
		t.Errorf("absolute finding passes through: got %q", got)
	}
}

func TestRenderRejectsUnknownFormat(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := render(&out, nil, "yaml", ".")
	if err == nil || !strings.Contains(err.Error(), "invalid --output") {
		t.Fatalf("want invalid --output error, got %v", err)
	}
}

func TestScanCommand(t *testing.T) {
	badshop := filepath.Join("..", "fixtures", "_badshop")
	if _, err := os.Stat(badshop); err != nil {
		t.Skip("fixtures not available")
	}
	godeep := filepath.Join("..", "fixtures", "godeep")
	clean := t.TempDir()
	if err := os.WriteFile(filepath.Join(clean, "README.md"), []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("pattern scan on fixture finds issues", func(t *testing.T) {
		root := NewRootCommand()
		buf := &bytes.Buffer{}
		root.SetOut(buf)
		root.SetArgs([]string{"scan", "--engine", "pattern", "--output", "json", badshop})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), `"rule"`) {
			t.Fatalf("expected findings JSON, got %q", buf.String())
		}
	})

	t.Run("go engine on godeep", func(t *testing.T) {
		root := NewRootCommand()
		buf := &bytes.Buffer{}
		root.SetOut(buf)
		root.SetArgs([]string{"scan", "--engine", "go", "--output", "text", godeep})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "findings") {
			t.Fatalf("expected text summary, got %q", buf.String())
		}
	})

	t.Run("clean tree with fail-on stays quiet", func(t *testing.T) {
		root := NewRootCommand()
		buf := &bytes.Buffer{}
		root.SetOut(buf)
		root.SetArgs([]string{"scan", "--engine", "pattern", "--fail-on", "warning", clean})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "0 findings") {
			t.Fatalf("got %q", buf.String())
		}
	})

	t.Run("invalid engine", func(t *testing.T) {
		root := NewRootCommand()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"scan", "--engine", "wasm", clean})
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "invalid --engine") {
			t.Fatalf("want invalid --engine, got %v", err)
		}
	})

	t.Run("invalid fail-on", func(t *testing.T) {
		root := NewRootCommand()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"scan", "--fail-on", "loud", clean})
		err := root.Execute()
		if err == nil || !strings.Contains(err.Error(), "invalid --fail-on") {
			t.Fatalf("want invalid --fail-on, got %v", err)
		}
	})

	t.Run("fail-on trips on findings", func(t *testing.T) {
		root := NewRootCommand()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"scan", "--engine", "pattern", "--fail-on", "warning", badshop})
		err := root.Execute()
		if err == nil {
			t.Fatal("want fail-on threshold error")
		}
		var thresh failOnThresholdError
		if !errors.As(err, &thresh) && !strings.Contains(err.Error(), "findings at or above") {
			t.Fatalf("want threshold error, got %v", err)
		}
	})

	t.Run("explicit rules config", func(t *testing.T) {
		cfg, err := filepath.Abs(filepath.Join("..", "sgconfig.yml"))
		if err != nil {
			t.Fatal(err)
		}
		root := NewRootCommand()
		buf := &bytes.Buffer{}
		root.SetOut(buf)
		root.SetArgs([]string{"scan", "--rules", cfg, "--engine", "pattern", clean})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("html output", func(t *testing.T) {
		root := NewRootCommand()
		buf := &bytes.Buffer{}
		root.SetOut(buf)
		root.SetArgs([]string{"scan", "--engine", "pattern", "--output", "html", clean})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "<!DOCTYPE html>") {
			t.Fatalf("want html, got %q", buf.String())
		}
	})
}

func writeSource(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func lifecycleFinding(file string, line int) finding.Finding {
	f := finding.Finding{
		Rule:     "rule/a",
		Severity: "warning",
		Location: finding.Location{File: file, Line: line, Symbol: "s"},
		Message:  "m",
		Status:   finding.StatusOpen,
	}
	f.Fill()
	return f
}

func TestApplyLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("no flags passes everything through", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeSource(t, dir, "package p\n")
		in := []finding.Finding{lifecycleFinding("a.go", 1)}

		out, suppressed, known, err := applyLifecycle(dir, in, "", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 1 || suppressed != 0 || known != 0 {
			t.Fatalf("got %d out, %d suppressed, %d known; want 1/0/0", len(out), suppressed, known)
		}
	})

	t.Run("suppression marks but preserves", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeSource(t, dir, "package p\n\n// tarsier:ignore rule/a accepted risk for now\nx := 1 // tarsier:ignore rule/a same line\n\ny := 2\n")
		in := []finding.Finding{
			lifecycleFinding("a.go", 3),
			lifecycleFinding("a.go", 4),
			lifecycleFinding("a.go", 6),
		}

		out, suppressed, known, err := applyLifecycle(dir, in, "", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 3 || suppressed != 2 || known != 0 {
			t.Fatalf("got %d out, %d suppressed, %d known; want 3/2/0", len(out), suppressed, known)
		}
		for _, f := range out[:2] {
			if f.Status != finding.StatusSuppressed {
				t.Errorf("suppressed finding lost its status: %+v", f)
			}
		}
		for i := range in {
			if in[i].Status != finding.StatusOpen {
				t.Errorf("applyLifecycle must not mutate input findings, index %d status %q", i, in[i].Status)
			}
		}
	})

	t.Run("baseline reports only new findings", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeSource(t, dir, "package p\n")
		old := lifecycleFinding("a.go", 1)
		fresh := lifecycleFinding("b.go", 2)
		base := filepath.Join(dir, "baseline.json")
		if err := finding.WriteBaseline(base, []finding.Finding{old}); err != nil {
			t.Fatal(err)
		}

		out, _, known, err := applyLifecycle(dir, []finding.Finding{old, fresh}, base, "")
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 1 || out[0].Fingerprint != fresh.Fingerprint || known != 1 {
			t.Fatalf("got %+v with %d known; want only the fresh finding", out, known)
		}
	})

	t.Run("baseline write records open findings", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeSource(t, dir, "package p\n")
		in := []finding.Finding{lifecycleFinding("a.go", 1)}
		base := filepath.Join(dir, "baseline.json")

		if _, _, _, err := applyLifecycle(dir, in, "", base); err != nil {
			t.Fatal(err)
		}
		set, err := finding.LoadBaseline(base)
		if err != nil {
			t.Fatal(err)
		}
		if !set[in[0].Fingerprint] {
			t.Fatalf("written baseline misses the finding: %v", set)
		}
	})

	t.Run("same baseline path reads before it writes", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		writeSource(t, dir, "package p\n")
		old := lifecycleFinding("a.go", 1)
		fresh := lifecycleFinding("b.go", 2)
		base := filepath.Join(dir, "baseline.json")
		if err := finding.WriteBaseline(base, []finding.Finding{old}); err != nil {
			t.Fatal(err)
		}

		out, _, known, err := applyLifecycle(dir, []finding.Finding{old, fresh}, base, base)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 1 || out[0].Fingerprint != fresh.Fingerprint || known != 1 {
			t.Fatalf("same-path must report against the previous set, got %+v with %d known", out, known)
		}
		set, err := finding.LoadBaseline(base)
		if err != nil {
			t.Fatal(err)
		}
		if !set[old.Fingerprint] || !set[fresh.Fingerprint] {
			t.Fatalf("same-path must then record the current set: %v", set)
		}
	})

	t.Run("missing baseline path is an error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		in := []finding.Finding{lifecycleFinding("a.go", 1)}

		if _, _, _, err := applyLifecycle(dir, in, filepath.Join(dir, "nope.json"), ""); err == nil {
			t.Fatal("a typo'd baseline path must error, not silently report everything as new")
		}
	})
}

func TestRootCommandRejectsAnUnknownOutput(t *testing.T) {
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"version", "--output", "jsonl"})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid --output") {
		t.Fatalf("a typo'd format must not silently print text, got %v", err)
	}
}
