// Package pattern drives ast-grep, the multi-language pattern tier, and
// normalizes its matches into tarsier findings.
package pattern

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Pieczasz/tarsier/internal/engine/resolve"
	"github.com/Pieczasz/tarsier/internal/finding"
)

const (
	defaultBin          = "ast-grep"
	defaultTimeout      = 5 * time.Minute
	versionCheckTimeout = 30 * time.Second
	minVersion          = "0.45.0"
)

// Runner scans a directory tree with the project's ast-grep rule set.
type Runner struct {
	Bin     string        // ast-grep executable; defaults to "ast-grep" on PATH
	Config  string        // path to sgconfig.yml
	Timeout time.Duration // per-scan wall clock; defaults to 5 minutes
}

// Scan runs every configured rule over root and returns normalized findings.
func (r *Runner) Scan(ctx context.Context, root string) ([]finding.Finding, error) {
	path, err := r.resolveBin()
	if err != nil {
		return nil, err
	}
	dir, target, err := resolveDirTarget(root)
	if err != nil {
		return nil, err
	}

	// Version probe uses its own budget so a slow --version (NFS PATH, etc.)
	// cannot steal from the scan timeout the caller configured.
	verCtx, verCancel := context.WithTimeout(ctx, versionCheckTimeout)
	err = checkVersion(verCtx, path)
	verCancel()
	if err != nil {
		return nil, err
	}

	timeout := r.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args, err := r.buildArgs(target)
	if err != nil {
		return nil, err
	}

	findings, err := executeScan(ctx, path, dir, root, args)
	if err != nil {
		return nil, err
	}
	extras, err := resolve.CardinalityExtras(root)
	if err != nil {
		return nil, err
	}
	if len(extras) == 0 {
		return findings, nil
	}
	findings = append(findings, extras...)
	slices.SortFunc(findings, func(a, b finding.Finding) int {
		if c := strings.Compare(a.Location.File, b.Location.File); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Location.Line, b.Location.Line); c != 0 {
			return c
		}
		return strings.Compare(a.Fingerprint, b.Fingerprint)
	})
	return findings, nil
}

func (r *Runner) resolveBin() (string, error) {
	bin := r.Bin
	if bin == "" {
		bin = defaultBin
	}
	path, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("%s not found on PATH: install it with `brew install ast-grep` or `npm i -g @ast-grep/cli`", bin)
	}
	return path, nil
}

func resolveDirTarget(root string) (dir, target string, err error) {
	info, err := os.Stat(root)
	if err != nil {
		return "", "", fmt.Errorf("cannot scan %s: %w", root, err)
	}
	dir, target = root, "."
	if !info.IsDir() {
		dir, target = filepath.Dir(root), filepath.Base(root)
	}
	return dir, target, nil
}

func (r *Runner) buildArgs(target string) ([]string, error) {
	args := []string{"scan", "--json=stream", "--include-metadata"}
	args = append(args, excludeGlobs()...)
	scanArgs, err := r.configArgs()
	if err != nil {
		return nil, err
	}
	args = append(args, scanArgs...)
	return append(args, target), nil
}

func executeScan(ctx context.Context, path, dir, root string, args []string) ([]finding.Finding, error) {
	cmd := exec.CommandContext(ctx, path, args...) //nolint:gosec // G204: not user input
	cmd.Dir = dir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("ast-grep stdout: %w", err)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start ast-grep: %w", err)
	}

	findings, decodeErr := decode(stdout, root, dir)
	// Drain anything left so the child never blocks writing to a full pipe.
	_, _ = io.Copy(io.Discard, stdout)

	if err := waitErr(ctx, cmd.Wait(), &stderr, len(findings) > 0); err != nil {
		return nil, err
	}
	if decodeErr != nil {
		if len(findings) == 0 {
			return nil, decodeErr
		}
		slog.Warn("ast-grep output ended mid-stream", "root", root, "detail", decodeErr.Error())
	}

	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		slog.Warn("ast-grep skipped part of the tree", "root", root, "detail", msg)
	}

	slices.SortFunc(findings, func(a, b finding.Finding) int {
		if c := strings.Compare(a.Location.File, b.Location.File); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Location.Line, b.Location.Line); c != 0 {
			return c
		}
		return strings.Compare(a.Fingerprint, b.Fingerprint)
	})
	return findings, nil
}

func (r *Runner) configArgs() ([]string, error) {
	if r.Config == "" {
		return nil, nil
	}
	config, err := filepath.Abs(r.Config)
	if err != nil {
		return nil, fmt.Errorf("resolve config %s: %w", r.Config, err)
	}
	return []string{"--config", config}, nil
}

func checkVersion(ctx context.Context, path string) error {
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return fmt.Errorf("ast-grep --version: %w", err)
	}
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return fmt.Errorf("cannot parse ast-grep version from %q", strings.TrimSpace(string(out)))
	}
	if !versionAtLeast(fields[1], minVersion) {
		return fmt.Errorf("ast-grep %s is too old: tarsier needs >= %s", fields[1], minVersion)
	}
	return nil
}

func versionAtLeast(got, want string) bool {
	g, w := parseVersion(got), parseVersion(want)
	for i := range w {
		if g[i] != w[i] {
			return g[i] > w[i]
		}
	}
	return true
}

func parseVersion(s string) [3]int {
	var v [3]int
	for i, part := range strings.SplitN(strings.TrimPrefix(s, "v"), ".", 3) {
		if i > 2 {
			break
		}
		n, _ := strconv.Atoi(strings.TrimFunc(part, func(r rune) bool { return r < '0' || r > '9' }))
		v[i] = n
	}
	return v
}

func decode(r io.Reader, root, dir string) ([]finding.Finding, error) {
	if dir == "" {
		dir = root
	}
	dec := json.NewDecoder(r)
	seen := map[string]int{}
	var out []finding.Finding
	excludedCount := 0
	for {
		var m match
		if err := dec.Decode(&m); err != nil {
			if errors.Is(err, io.EOF) {
				if excludedCount > 0 {
					slog.Debug("ast-grep excluded findings", "excluded", excludedCount, "root", root)
				}
				return out, nil
			}
			if excludedCount > 0 {
				slog.Debug("ast-grep excluded findings", "excluded", excludedCount, "root", root)
			}
			return out, fmt.Errorf("decode ast-grep output: %w", err)
		}
		f := m.toFinding(root, seen)
		// Compound-token post-filter only (see excluded). Exact vendor/test
		// directory names and suffixes are skipped earlier via --globs.
		// Match on the path relative to the scan dir so an excluded name
		// above the root does not empty an explicit scan underneath it.
		if excluded(relPath(dir, m.File)) {
			excludedCount++
			continue
		}
		out = append(out, f)
	}
}

func (m *match) toFinding(root string, seen map[string]int) finding.Finding {
	rule := m.meta("rule")
	if rule == "" {
		rule = m.RuleID
	}
	rel := relPath(root, m.File)

	static := map[string]any{
		"matched":       m.Text,
		"ast_grep_rule": m.RuleID,
	}
	for _, key := range []string{"requires", "expectation_layer", "spec_ref"} {
		if v := m.meta(key); v != "" {
			static[key] = v
		}
	}

	display := discriminator(rule, rel, m.Text, seen)
	// Fingerprint stays on the full match text so display truncation never
	// invalidates baselines.
	full := m.Text
	if n := seen[rule+"|"+rel+"|"+m.Text]; n > 1 {
		full = fmt.Sprintf("%s#%d", m.Text, n)
	}

	f := finding.Finding{
		Rule:       rule,
		Language:   strings.ToLower(m.Language),
		Severity:   m.Severity,
		Confidence: m.meta("confidence"),
		Location: finding.Location{
			File: rel,
			// ast-grep counts lines from zero
			Line:   m.Range.Start.Line + 1,
			Symbol: display,
		},
		Message:  m.Message,
		Note:     strings.TrimSpace(m.Note),
		Evidence: finding.Evidence{Static: static},
		Status:   finding.StatusOpen,
	}
	f.Fingerprint = finding.Fingerprint(f.Rule, f.Location.Module, f.Location.File, full)
	return f
}

func relPath(root, file string) string {
	// Single-file scans report the basename relative to the parent dir,
	// so preserve the caller's input path instead of collapsing to basename.
	if file != "" && !strings.Contains(file, "/") && !strings.Contains(file, "\\") {
		if filepath.Base(root) == file {
			return filepath.ToSlash(root)
		}
	}
	if rel, err := filepath.Rel(root, file); err == nil {
		if rel == "." {
			return filepath.ToSlash(root)
		}
		if !strings.HasPrefix(rel, "..") {
			file = rel
		}
	}
	return filepath.ToSlash(file)
}

const maxSymbolLen = 120

func truncateBase(s string) string {
	if !strings.Contains(s, "\n") && len(s) <= maxSymbolLen {
		return s
	}
	single := strings.Join(strings.Fields(s), " ")
	if len(single) <= maxSymbolLen {
		return single
	}
	runes := []rune(single)
	if len(runes) <= maxSymbolLen {
		return single
	}
	return string(runes[:maxSymbolLen-1]) + "…"
}

func discriminator(rule, rel, text string, seen map[string]int) string {
	key := rule + "|" + rel + "|" + text
	seen[key]++
	display := truncateBase(text)
	if n := seen[key]; n > 1 {
		return fmt.Sprintf("%s#%d", display, n)
	}
	return display
}

// Path exclusions are evidence-driven (TAR-18 / corpus), not imagined.
// Compound names without '-'|'_' must be listed whole: markerTokens only
// splits on those separators, so "testmocks" does not match "test"/"mocks".
var (
	excludedDirs = []string{
		"vendor", "node_modules", "third_party", "vendored", "_vendor",
		"dist", "build", "target", "generated", "gen",
		"integration", "scripts", "tools", "buildtools", "hack",
		"__tests__", "__mocks__",
	}
	markerTokens = []string{
		"test", "tests", "testing", "testdata", "mock", "mocks",
		"fixture", "fixtures", "example", "examples", "demo", "sample",
		"samples", "e2e", "acceptance", "benchmark", "benchmarks",
		// Whole-segment compounds seen in corpus trees; "testmocks" leaked
		// 6 errors/swallowed findings on apache/pulsar before this entry.
		"testmocks", "testhelpers", "testutil", "testutils",
		"dbmock", "integrationtest",
	}
	excludedSuffixes = []string{
		"_test.go",
		".test.ts", ".test.tsx", ".test.js", ".test.jsx",
		".spec.ts", ".spec.tsx", ".spec.js", ".spec.jsx",
		"Test.java", "Tests.java", "IT.java",
	}
)

// excludeGlobs is the primary exclusion mechanism: exact directory names
// (vendor, test, testmocks, …) and file suffixes never reach ast-grep.
func excludeGlobs() []string {
	args := make([]string, 0, 2*(len(excludedDirs)+len(markerTokens)+len(excludedSuffixes)))
	for _, dir := range slices.Concat(excludedDirs, markerTokens) {
		args = append(args, "--globs", "!**/"+dir+"/**")
	}
	for _, suffix := range excludedSuffixes {
		args = append(args, "--globs", "!**/*"+suffix)
	}
	return args
}

// excluded is the residual post-filter for compound path segments only.
//
// --globs can express !**/test/** but not "any directory whose name
// contains the token test" (kubernetes-tests, go_example). Those need a
// hyphen/underscore split after the fact. Exact directory names and
// suffixes are intentionally not re-checked here - one mechanism each.
func excluded(path string) bool {
	for part := range strings.SplitSeq(path, "/") {
		if !strings.ContainsAny(part, "-_") {
			continue
		}
		for token := range strings.FieldsFuncSeq(part, func(r rune) bool { return r == '-' || r == '_' }) {
			if slices.Contains(markerTokens, token) {
				return true
			}
		}
	}
	return false
}

func waitErr(ctx context.Context, err error, stderr *strings.Builder, matched bool) error {
	if err == nil {
		return nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("ast-grep timed out: %w", ctx.Err())
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return fmt.Errorf("ast-grep canceled: %w", ctx.Err())
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 && matched {
		return nil
	}
	if msg := strings.TrimSpace(stderr.String()); msg != "" {
		return fmt.Errorf("ast-grep failed: %w: %s", err, msg)
	}
	return fmt.Errorf("ast-grep failed: %w", err)
}
