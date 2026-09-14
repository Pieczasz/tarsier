package report

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/Pieczasz/tarsier/finding"
)

func TestWriteHTMLGroupsByRuleAndEscapes(t *testing.T) {
	t.Parallel()

	findings := []finding.Finding{
		{
			Rule:       "metrics/high-cardinality-label",
			Language:   "go",
			Severity:   "warning",
			Confidence: "high",
			Location:   finding.Location{File: "api/metrics.go", Line: 14},
			Message:    `Metric label "user_id" is unbounded`,
			Note:       "Drop the label.",
			Status:     finding.StatusOpen,
			Evidence: finding.Evidence{Static: map[string]any{
				"matched":           `"user_id"`,
				"expectation_layer": "2",
				"spec_ref":          "instrumentation-score/MET-001",
			}},
		},
		{
			Rule:     "logs/unstructured-logging",
			Language: "go",
			Severity: "warning",
			Location: finding.Location{File: "api/cli.go", Line: 9},
			Message:  "unstructured <log>",
			Status:   finding.StatusSuppressed,
		},
	}

	s := Build("/repo", findings, time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC))
	if s.Open != 1 || s.Suppressed != 1 {
		t.Fatalf("open=%d suppressed=%d; want 1/1", s.Open, s.Suppressed)
	}
	if len(s.Rules) != 2 || s.Rules[0].Rule != "logs/unstructured-logging" {
		t.Fatalf("rules should be sorted: %+v", s.Rules)
	}

	var buf bytes.Buffer
	if err := WriteHTML(&buf, &s); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	for _, want := range []string{
		"tarsier scan",
		"api/metrics.go:14",
		"Drop the label.",
		"unstructured &lt;log&gt;",
		"suppressed",
		"no CDN",
		"Instrumentation Score",
	} {
		if want == "no CDN" {
			if strings.Contains(html, "http://") || strings.Contains(html, "https://") {
				t.Errorf("report must be offline; found a URL:\n%s", html)
			}
			continue
		}
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestWriteHTMLEmptyScan(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	s := Build(".", nil, time.Unix(0, 0).UTC())
	if err := WriteHTML(&buf, &s); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No findings.") {
		t.Fatalf("empty scan should say so:\n%s", buf.String())
	}
}
