package finding

import (
	"strings"
	"testing"
)

func suppressedFinding(line int) Finding {
	return Finding{
		Rule:     "rule/a",
		Location: Location{File: "f.go", Line: line, Symbol: "s"},
		Status:   StatusOpen,
	}
}

func TestParseSuppressionsReadsAllThreeCommentStyles(t *testing.T) {
	t.Parallel()

	src := []byte("// tarsier:ignore rule/a reason one\n" +
		"# tarsier:ignore rule/b reason two\n" +
		"-- tarsier:ignore rule/c reason three\n")

	got := ParseSuppressions("f.go", src)
	if len(got) != 3 {
		t.Fatalf("parsed %d suppressions, want 3: %+v", len(got), got)
	}
	if got[0].Rule != "rule/a" || got[0].Reason != "reason one" || got[0].Line != 1 {
		t.Errorf("first suppression wrong: %+v", got[0])
	}
}

func TestParseSuppressionsRejectsAReasonlessComment(t *testing.T) {
	t.Parallel()

	src := []byte("x := 1 // tarsier:ignore rule/a\n" +
		"y := 2 // tarsier:ignore rule/a   \n")

	if got := ParseSuppressions("f.go", src); len(got) != 0 {
		t.Fatalf("reasonless suppressions must not parse, got %+v", got)
	}
}

func TestParseSuppressionsSurvivesLongLines(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 100*1024)
	src := []byte(long + "\n// tarsier:ignore rule/a past a minified line\n")

	got := ParseSuppressions("f.go", src)
	if len(got) != 1 || got[0].Line != 2 {
		t.Fatalf("suppression past a >64k line lost (bufio default cap): %+v", got)
	}
}

func TestParseSuppressionsFindsTrailingComments(t *testing.T) {
	t.Parallel()

	src := []byte("package p\n\nx := r.ReadMessage(ctx) // tarsier:ignore rule/a trailing reason\n")

	got := ParseSuppressions("f.go", src)
	if len(got) != 1 || got[0].Line != 3 || got[0].Rule != "rule/a" || got[0].Reason != "trailing reason" {
		t.Fatalf("trailing suppression not parsed: %+v", got)
	}

	findings := []Finding{suppressedFinding(3)}
	kept, suppressed := ApplySuppressions(findings, got)
	if len(kept) != 0 || len(suppressed) != 1 {
		t.Fatalf("trailing comment must suppress its own line without a line-above comment: %+v / %+v", kept, suppressed)
	}
}

func TestApplySuppressionsDoesNotMutateInput(t *testing.T) {
	t.Parallel()

	findings := []Finding{suppressedFinding(10)}
	sup := []Suppression{{File: "f.go", Line: 10, Rule: "rule/a", Reason: "accepted"}}

	kept, suppressed := ApplySuppressions(findings, sup)
	if len(kept) != 0 || len(suppressed) != 1 {
		t.Fatalf("want 0 kept / 1 suppressed, got %d / %d", len(kept), len(suppressed))
	}
	if findings[0].Status != StatusOpen {
		t.Fatalf("ApplySuppressions must not mutate the input slice, got status %q", findings[0].Status)
	}
	if suppressed[0].Status != StatusSuppressed {
		t.Fatalf("returned suppressed finding must carry StatusSuppressed, got %q", suppressed[0].Status)
	}
}

func TestApplySuppressionsMatchesSameLineAndLineAbove(t *testing.T) {
	t.Parallel()

	findings := []Finding{
		suppressedFinding(10),
		suppressedFinding(20),
		suppressedFinding(30),
	}
	sup := []Suppression{
		{File: "f.go", Line: 10, Rule: "rule/a", Reason: "same line"},
		{File: "f.go", Line: 19, Rule: "rule/a", Reason: "line above"},
		{File: "f.go", Line: 28, Rule: "rule/a", Reason: "too far above"},
	}

	kept, suppressed := ApplySuppressions(findings, sup)

	if len(suppressed) != 2 {
		t.Fatalf("suppressed %d findings, want 2", len(suppressed))
	}
	for _, f := range suppressed {
		if f.Status != StatusSuppressed {
			t.Errorf("suppressed finding lost its status: %+v", f)
		}
	}
	if len(kept) != 1 || kept[0].Location.Line != 30 {
		t.Fatalf("kept set wrong: %+v", kept)
	}
	if kept[0].Status != StatusOpen {
		t.Errorf("kept finding status changed: %+v", kept[0])
	}
}

func TestApplySuppressionsRequiresTheFile(t *testing.T) {
	t.Parallel()

	a := suppressedFinding(10)
	a.Location.File = "a.go"
	b := suppressedFinding(10)
	b.Location.File = "b.go"
	sup := []Suppression{{File: "a.go", Line: 10, Rule: "rule/a", Reason: "only a"}}

	kept, suppressed := ApplySuppressions([]Finding{a, b}, sup)
	if len(suppressed) != 1 || suppressed[0].Location.File != "a.go" {
		t.Fatalf("suppression leaked across files: kept %+v / suppressed %+v", kept, suppressed)
	}
	if len(kept) != 1 || kept[0].Location.File != "b.go" {
		t.Fatalf("wrong file kept: %+v", kept)
	}
}

func TestApplySuppressionsRequiresTheRuleID(t *testing.T) {
	t.Parallel()

	findings := []Finding{suppressedFinding(10)}
	sup := []Suppression{{File: "f.go", Line: 10, Rule: "rule/b", Reason: "other rule"}}

	kept, suppressed := ApplySuppressions(findings, sup)
	if len(kept) != 1 || len(suppressed) != 0 {
		t.Fatalf("wrong-rule suppression must not apply: %+v / %+v", kept, suppressed)
	}
}
