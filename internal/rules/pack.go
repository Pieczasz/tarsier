// Package rules embeds the ast-grep rule pack so a scan works from any
// directory, not just a tarsier checkout.
package rules

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/Pieczasz/tarsier/internal/ruleutils"
)

// Pattern, not a category list, so a new rule directory ships without an
// edit here. One level only: */*.yml does not recurse, so a future nested
// subdir needs its own pattern.
//
//go:embed */*.yml
var packFS embed.FS

const packConfig = `ruleDirs:
  - rules
utilDirs:
  - utils
`

// Materialize writes the embedded rule pack under dir and returns the path of
// an sgconfig.yml that points at it.
func Materialize(dir string) (string, error) {
	if err := os.CopyFS(filepath.Join(dir, "rules"), packFS); err != nil {
		return "", err
	}
	if err := os.CopyFS(filepath.Join(dir, "utils"), ruleutils.FS); err != nil {
		return "", err
	}
	config := filepath.Join(dir, "sgconfig.yml")
	if err := os.WriteFile(config, []byte(packConfig), 0o600); err != nil {
		return "", err
	}
	return config, nil
}
