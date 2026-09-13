// Package ruleutils holds the shared ast-grep utility rules referenced by the
// rule pack via `matches:`.
package ruleutils

import "embed"

// FS carries the utility rule files for embedding into the binary.
//
//go:embed *.yml
var FS embed.FS
