package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMaterializeWritesPackAndConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	config, err := Materialize(dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(config) != "sgconfig.yml" {
		t.Fatalf("config path = %q", config)
	}
	body, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "ruleDirs:") || !strings.Contains(string(body), "utilDirs:") {
		t.Fatalf("sgconfig content unexpected: %s", body)
	}
	rulesDir := filepath.Join(dir, "rules")
	entries, err := os.ReadDir(rulesDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("expected embedded rule directories under rules/")
	}
	utilsDir := filepath.Join(dir, "utils")
	if _, err := os.Stat(filepath.Join(utilsDir, "unbounded-label.yml")); err != nil {
		t.Fatalf("embedded util missing: %v", err)
	}
}

func TestMaterializeRejectsNonDirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Materialize(path); err == nil {
		t.Fatal("Materialize into a file path must fail")
	}
}

func TestMaterializeRejectsUtilsPathConflict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "utils"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Materialize(dir); err == nil {
		t.Fatal("Materialize must fail when utils is a file")
	}
}

func TestMaterializeRejectsConfigPathConflict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sgconfig.yml"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Materialize(dir); err == nil {
		t.Fatal("Materialize must fail when sgconfig.yml is a directory")
	}
}
