// Package report renders a self-contained HTML scan report.
package report

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/Pieczasz/tarsier/finding"
)

//go:embed report.html
var htmlSrc string

var htmlTmpl = template.Must(template.New("report").Parse(htmlSrc))

// Summary is the data the HTML template renders.
type Summary struct {
	Root       string
	Generated  string
	Open       int
	Suppressed int
	Rules      []RuleGroup
	Languages  []namedCount
	Severities []namedCount
}

// RuleGroup is one logical rule and the findings it produced.
type RuleGroup struct {
	Rule     string
	Count    int
	Findings []Row
}

// Row is one finding as the template sees it.
type Row struct {
	File       string
	Line       int
	Severity   string
	Confidence string
	Layer      string
	Language   string
	Message    string
	Note       string
	Status     string
	Matched    string
	SpecRef    string
	Suppressed bool
}

type namedCount struct {
	Name  string
	Count int
}

// Build groups findings by rule for the HTML report.
func Build(root string, findings []finding.Finding, now time.Time) Summary {
	s := Summary{
		Root:      root,
		Generated: now.UTC().Format(time.RFC3339),
	}
	byRule := map[string][]Row{}
	lang := map[string]int{}
	sev := map[string]int{}
	for i := range findings {
		f := &findings[i]
		row := rowFrom(f)
		byRule[f.Rule] = append(byRule[f.Rule], row)
		if f.Status == finding.StatusSuppressed {
			s.Suppressed++
			continue
		}
		s.Open++
		if f.Language != "" {
			lang[f.Language]++
		}
		if f.Severity != "" {
			sev[f.Severity]++
		}
	}
	s.Rules = make([]RuleGroup, 0, len(byRule))
	for rule, rows := range byRule {
		s.Rules = append(s.Rules, RuleGroup{Rule: rule, Count: len(rows), Findings: rows})
	}
	slices.SortFunc(s.Rules, func(a, b RuleGroup) int {
		return strings.Compare(a.Rule, b.Rule)
	})
	s.Languages = sortedCounts(lang)
	s.Severities = sortedCounts(sev)
	return s
}

func rowFrom(f *finding.Finding) Row {
	matched, _ := f.Evidence.Static["matched"].(string)
	layer, _ := f.Evidence.Static["expectation_layer"].(string)
	spec, _ := f.Evidence.Static["spec_ref"].(string)
	return Row{
		File:       f.Location.File,
		Line:       f.Location.Line,
		Severity:   f.Severity,
		Confidence: f.Confidence,
		Layer:      layer,
		Language:   f.Language,
		Message:    f.Message,
		Note:       f.Note,
		Status:     f.Status,
		Matched:    matched,
		SpecRef:    spec,
		Suppressed: f.Status == finding.StatusSuppressed,
	}
}

func sortedCounts(m map[string]int) []namedCount {
	out := make([]namedCount, 0, len(m))
	for name, n := range m {
		out = append(out, namedCount{Name: name, Count: n})
	}
	slices.SortFunc(out, func(a, b namedCount) int {
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

// WriteHTML writes a single-file HTML report to w. No network, no CDN.
func WriteHTML(w io.Writer, s *Summary) error {
	var buf bytes.Buffer
	if err := htmlTmpl.Execute(&buf, s); err != nil {
		return fmt.Errorf("render html report: %w", err)
	}
	_, err := w.Write(buf.Bytes())
	return err
}
