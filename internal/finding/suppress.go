package finding

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// Suppression is one parsed tarsier:ignore comment. Line is 1-based, matching
// finding.Location.Line.
type Suppression struct {
	File   string
	Line   int
	Rule   string
	Reason string
}

// maxSuppressLine caps a single source line at 1MB. bufio.Scanner stops at
// 64k by default and reports nothing, which would silently drop suppressions
// past a minified line.
const maxSuppressLine = 1024 * 1024

var suppressPattern = regexp.MustCompile(`(?:^\s*|\s+)(?://|#|--)\s*tarsier:ignore\s+(\S+)(?:\s+(.*?))?\s*$`)

// ParseSuppressions extracts suppressions from source bytes. A comment
// without a reason is not a suppression.
func ParseSuppressions(path string, src []byte) []Suppression {
	var out []Suppression
	scanner := bufio.NewScanner(bytes.NewReader(src))
	scanner.Buffer(make([]byte, 64*1024), maxSuppressLine)
	for line := 1; scanner.Scan(); line++ {
		m := suppressPattern.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		if strings.TrimSpace(m[2]) == "" {
			continue
		}
		out = append(out, Suppression{File: path, Line: line, Rule: m[1], Reason: strings.TrimSpace(m[2])})
	}
	return out
}

// ApplySuppressions partitions findings into kept and suppressed without
// mutating the input slice. A finding carrying a matching suppression on its
// own line or the line immediately above is copied into suppressed with
// StatusSuppressed; everything else is copied into kept unchanged.
func ApplySuppressions(findings []Finding, sup []Suppression) (kept, suppressed []Finding) {
	for i := range findings {
		f := findings[i]
		if suppressedBy(&f, sup) {
			f.Status = StatusSuppressed
			suppressed = append(suppressed, f)
		} else {
			kept = append(kept, f)
		}
	}
	return kept, suppressed
}

func suppressedBy(f *Finding, sup []Suppression) bool {
	for _, s := range sup {
		if s.File != f.Location.File || s.Rule != f.Rule {
			continue
		}
		if s.Line == f.Location.Line || s.Line == f.Location.Line-1 {
			return true
		}
	}
	return false
}
