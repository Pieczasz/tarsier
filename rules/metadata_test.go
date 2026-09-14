package rules

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

var fileSuffixToLanguage = map[string]string{
	"go":     "go",
	"ts":     "typescript",
	"java":   "java",
	"rust":   "rust",
	"python": "python",
}

var (
	validConfidence = []string{"high", "medium", "low"}
	validSeverity   = []string{"hint", "info", "warning", "error"}
)

type ruleFile struct {
	ID       string `yaml:"id"`
	Language string `yaml:"language"`
	Severity string `yaml:"severity"`
	Message  string `yaml:"message"`
	Note     string `yaml:"note"`
	Metadata struct {
		Rule             string `yaml:"rule"`
		Requires         string `yaml:"requires"`
		ExpectationLayer string `yaml:"expectation_layer"`
		Confidence       string `yaml:"confidence"`
		SpecRef          string `yaml:"spec_ref"`
	} `yaml:"metadata"`
}

func TestEveryRuleFileMatchesTheSchema(t *testing.T) {
	t.Parallel()

	paths := ruleFilePaths(t)
	seenIDs := make(map[string]string, len(paths))

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			category, ruleName, suffix := splitRulePath(t, path)

			var rule ruleFile
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal(raw, &rule); err != nil {
				t.Fatalf("not valid YAML: %v", err)
			}

			wantID := category + "-" + ruleName + "-" + suffix
			if rule.ID != wantID {
				t.Errorf("id = %q, want %q (derived from the path)", rule.ID, wantID)
			}
			if strings.Contains(rule.ID, "/") {
				t.Errorf("id %q contains a slash; ast-grep names snapshot files from the id and will not create the directory", rule.ID)
			}

			if want := fileSuffixToLanguage[suffix]; rule.Language != want {
				t.Errorf("language = %q, want %q for a %s.yml file", rule.Language, want, suffix)
			}

			if wantRule := category + "/" + ruleName; rule.Metadata.Rule != wantRule {
				t.Errorf("metadata.rule = %q, want %q; findings group across languages by this value", rule.Metadata.Rule, wantRule)
			}
			if rule.Metadata.Requires == "" {
				t.Error("metadata.requires is empty; it maps the finding to a workflow-matrix requirement")
			}
			if layer, err := strconv.Atoi(rule.Metadata.ExpectationLayer); err != nil || layer < 1 || layer > 4 {
				t.Errorf("metadata.expectation_layer = %q, want 1-4 (README 13.1)", rule.Metadata.ExpectationLayer)
			}
			if !slices.Contains(validConfidence, rule.Metadata.Confidence) {
				t.Errorf("metadata.confidence = %q, want one of %v", rule.Metadata.Confidence, validConfidence)
			}

			if !slices.Contains(validSeverity, rule.Severity) {
				t.Errorf("severity = %q, want one of %v", rule.Severity, validSeverity)
			}
			if rule.Message == "" {
				t.Error("message is empty; it is what the user reads first")
			}
			if rule.Note == "" {
				t.Error("note is empty; a finding without a fix suggestion gets ignored")
			}

			if previous, dup := seenIDs[rule.ID]; dup {
				t.Errorf("id %q already used by %s; ast-grep requires globally unique ids", rule.ID, previous)
			}
			seenIDs[rule.ID] = path
		})
	}
}

func TestEveryRuleHasTestCases(t *testing.T) {
	t.Parallel()

	for _, path := range ruleFilePaths(t) {
		t.Run(path, func(t *testing.T) {
			category, ruleName, suffix := splitRulePath(t, path)
			id := category + "-" + ruleName + "-" + suffix

			cases := filepath.Join("..", "fixtures", "ruletests", id+".yml")
			if _, err := os.Stat(cases); err != nil {
				t.Errorf("no test cases at %s: every rule needs valid/invalid snippets before it ships", cases)
			}
		})
	}
}

func ruleFilePaths(t *testing.T) []string {
	t.Helper()

	var paths []string
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".yml" {
			return err
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no rule files found")
	}
	return paths
}

func splitRulePath(t *testing.T, path string) (category, ruleName, suffix string) {
	t.Helper()

	category = filepath.Dir(path)
	if category == "." || strings.Contains(category, string(filepath.Separator)) {
		t.Fatalf("rule files live one directory deep, as <category>/<rule>.<lang>.yml, got %q", path)
	}

	base := strings.TrimSuffix(filepath.Base(path), ".yml")
	dot := strings.LastIndex(base, ".")
	if dot < 1 {
		t.Fatalf("filename must be <rule-name>.<lang>.yml, got %q", filepath.Base(path))
	}
	ruleName, suffix = base[:dot], base[dot+1:]

	if _, known := fileSuffixToLanguage[suffix]; !known {
		t.Fatalf("unknown language suffix %q; add it to fileSuffixToLanguage", suffix)
	}
	return category, ruleName, suffix
}
