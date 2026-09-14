// Package fixtures asserts the scanner against _badshop, a deliberately
// badly-instrumented application who's every interesting line is labeled.
package fixtures

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Pieczasz/tarsier/engine/pattern"
	"github.com/Pieczasz/tarsier/rules"
)

const fixtureRoot = "_badshop"

var markerPattern = regexp.MustCompile(`(?:#|//|--)\s*(want|notwant|gap):\s*(\S+)`)

type site struct {
	file string
	line int
	rule string
}

func TestBadshopMatchesItsMarkers(t *testing.T) {
	if _, err := exec.LookPath("ast-grep"); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("ast-grep must be on PATH in CI")
		}
		t.Skip("ast-grep not on PATH; `brew install ast-grep` to run fixture tests")
	}
	config, err := rules.Materialize(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	runner := &pattern.Runner{Config: config}
	found, err := runner.Scan(context.Background(), fixtureRoot)
	if err != nil {
		t.Fatal(err)
	}

	reported := make(map[site]bool, len(found))
	for _, f := range found {
		reported[site{f.Location.File, f.Location.Line, f.Rule}] = true
	}

	marked := readMarkers(t)
	var wants, notwants, gaps int

	for s, kind := range marked {
		switch kind {
		case "want":
			wants++
			if !reported[s] {
				t.Errorf("MISS  %s:%d %s - planted defect was not detected", s.file, s.line, s.rule)
			}
		case "notwant":
			notwants++
			if reported[s] {
				t.Errorf("FALSE POSITIVE  %s:%d %s - this shape must not be reported", s.file, s.line, s.rule)
			}
		case "gap":
			gaps++
			if reported[s] {
				t.Errorf("NEWLY DETECTED  %s:%d %s - good news: promote the `gap:` marker to `want:`", s.file, s.line, s.rule)
			}
		}
	}

	// A finding nobody asked for is a false positive
	for s := range reported {
		if _, ok := marked[s]; !ok {
			t.Errorf("UNEXPECTED  %s:%d %s - finding on a line with no marker", s.file, s.line, s.rule)
		}
	}

	t.Logf("badshop: %d detected defects, %d must-not-report shapes, %d known gaps", wants, notwants, gaps)
	if wants == 0 {
		t.Fatal("the fixture asserts nothing: a positive control with no positives cannot fail")
	}
}

func readMarkers(t *testing.T) map[site]string {
	t.Helper()
	markers := map[site]string{}

	err := filepath.WalkDir(fixtureRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if filepath.Ext(path) == ".md" {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(fixtureRoot, path)
		if err != nil {
			_ = f.Close()
			return err
		}
		rel = filepath.ToSlash(rel)

		scanner := bufio.NewScanner(f)
		for line := 1; scanner.Scan(); line++ {
			m := markerPattern.FindStringSubmatch(scanner.Text())
			if m == nil {
				continue
			}
			markers[site{rel, line, strings.TrimSuffix(m[2], ",")}] = m[1]
		}
		scanErr := scanner.Err()
		closeErr := f.Close()
		if scanErr != nil {
			return scanErr
		}
		return closeErr
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) == 0 {
		t.Fatalf("no markers found under %s/", fixtureRoot)
	}
	return markers
}
