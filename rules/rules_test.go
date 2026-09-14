package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The denylist of unbounded label names lives once, in
// ruleutils/unbounded-label.yml, reached via `matches:`.
//
// A rule that inlined its own copy would still pass its own fixtures, because
// each language's fixtures only exercise its own copy, so a drifted list would
// ship silently.
const sharedUtil = "is-unbounded-label"

var inlineDenylist = regexp.MustCompile(`\(user_id\|(?:[a-z_]+\|)+[a-z_]+\)`)

func TestDenylistIsDefinedExactlyOnce(t *testing.T) {
	t.Parallel()

	util := filepath.Join("..", "ruleutils", "unbounded-label.yml")
	src, err := os.ReadFile(util)
	if err != nil {
		t.Fatalf("the shared denylist must exist: %v", err)
	}
	if !inlineDenylist.Match(src) {
		t.Errorf("%s no longer contains a denylist alternation", util)
	}
	if !strings.Contains(string(src), "id: "+sharedUtil) {
		t.Errorf("%s must declare id %q", util, sharedUtil)
	}
}

func TestNoRuleInlinesADenylist(t *testing.T) {
	t.Parallel()

	var referencing int
	for _, file := range ruleFilePaths(t) {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		body := stripComments(string(src))

		if inlined := inlineDenylist.FindString(body); inlined != "" {
			t.Errorf("%s inlines %s; use `matches: %s`", file, inlined, sharedUtil)
		}
		if strings.Contains(body, "matches: "+sharedUtil) {
			referencing++
		}
	}
	if referencing == 0 {
		t.Errorf("no rule references %q, so the shared util is dead", sharedUtil)
	}
}

func stripComments(src string) string {
	var b strings.Builder
	for line := range strings.SplitSeq(src, "\n") {
		if trimmed := strings.TrimSpace(line); !strings.HasPrefix(trimmed, "#") {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}
