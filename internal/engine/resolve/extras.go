package resolve

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Pieczasz/tarsier/internal/finding"
)

var (
	javaLabelNamesRE = regexp.MustCompile(`\.(?:labelNames|tags)\s*\(\s*([A-Za-z_]\w*)\s*\)`)
	pyLabelsStrRE    = regexp.MustCompile(`\.labels\s*\(\s*str\s*\(\s*([A-Za-z_]\w*)\s*\)\s*\)`)
)

const (
	ruleHighCard  = "metrics/high-cardinality-label"
	ruleUnbounded = "metrics/unbounded-label-value"
	noteVarLabels = "Label names held in a file-level constant are resolved by the thin symbol layer (TAR-20). Drop the unbounded label or replace it with a bounded dimension."
	noteStrExc    = "str(exc) of an Exception-typed name is unbounded as a metric label. Bucket the error class; put the message on a span."
)

// CardinalityExtras walks root and emits findings the pattern tier cannot
// see: Java const-array label names and Python str(Exception) label values.
func CardinalityExtras(root string) ([]finding.Finding, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("resolve walk %s: %w", root, err)
	}
	walkRoot := root
	if !info.IsDir() {
		src, err := os.ReadFile(root) //nolint:gosec // G304: path is the scan root chosen by the user
		if err != nil {
			return nil, fmt.Errorf("resolve read %s: %w", root, err)
		}
		return extrasForFile(root, filepath.Base(root), src)
	}

	rootAbs, err := filepath.Abs(walkRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve abs %s: %w", walkRoot, err)
	}

	var out []finding.Finding
	err = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, err error) error {
		got, skip, walkErr := visitCardinalityPath(walkRoot, rootAbs, path, d, err)
		if walkErr != nil || skip {
			return walkErr
		}
		out = append(out, got...)
		return nil
	})
	return out, err
}

// visitCardinalityPath handles one WalkDir entry. skip means the entry was
// ignored (dir skip, wrong lang, symlink, escape). Findings may be empty.
func visitCardinalityPath(walkRoot, rootAbs, path string, d fs.DirEntry, err error) ([]finding.Finding, bool, error) {
	if err != nil {
		return nil, true, err
	}
	// Untrusted PR trees may plant symlinks that escape the checkout.
	if d.Type()&fs.ModeSymlink != 0 {
		if d.IsDir() {
			return nil, true, filepath.SkipDir
		}
		return nil, true, nil
	}
	if d.IsDir() {
		base := d.Name()
		if base == "vendor" || base == "node_modules" || base == ".git" {
			return nil, true, filepath.SkipDir
		}
		return nil, true, nil
	}
	lang := LangOf(path)
	if lang != langJava && lang != langPython {
		return nil, true, nil
	}
	if !pathInsideRoot(rootAbs, path) {
		return nil, true, nil
	}
	src, readErr := os.ReadFile(path) //nolint:gosec // G304: path verified under scan root
	if readErr != nil {
		return nil, true, nil //nolint:nilerr // best-effort skip of unreadable files
	}
	rel, relErr := filepath.Rel(walkRoot, path)
	if relErr != nil {
		rel = path
	}
	got, _ := extrasForFile(path, filepath.ToSlash(rel), src)
	return got, false, nil
}

// pathInsideRoot reports whether path resolves inside rootAbs (both absolute).
// Symlink targets that escape the scan root are rejected.
func pathInsideRoot(rootAbs, path string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		// Dangling or unresolvable: do not read.
		return false
	}
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		rootResolved = rootAbs
	}
	rel, err := filepath.Rel(rootResolved, resolved)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func extrasForFile(abs, rel string, src []byte) ([]finding.Finding, error) {
	f := Analyze(abs, src)
	switch f.Lang {
	case langJava:
		return javaConstLabelFindings(f, rel, string(src)), nil
	case langPython:
		return pythonStrExcFindings(f, rel, string(src)), nil
	default:
		return nil, nil
	}
}

func javaConstLabelFindings(f *File, rel, src string) []finding.Finding {
	used := map[string]bool{}
	for _, m := range javaLabelNamesRE.FindAllStringSubmatch(src, -1) {
		used[m[1]] = true
	}
	if len(used) == 0 {
		return nil
	}
	var out []finding.Finding
	for name, lits := range f.Strings {
		if !used[name] {
			continue
		}
		for _, lit := range lits {
			if !UnboundedLabel(lit.Value) {
				continue
			}
			out = append(out, finding.Finding{
				Rule:       ruleHighCard,
				Language:   "java",
				Severity:   "warning",
				Confidence: "high",
				Location: finding.Location{
					File:   rel,
					Line:   lit.Line,
					Symbol: lit.Value,
				},
				Message: fmt.Sprintf("Metric label %q is unbounded; each distinct value mints a new billable time series.", lit.Value),
				Note:    noteVarLabels,
				Evidence: finding.Evidence{Static: map[string]any{
					"matched":   lit.Value,
					"via_const": name,
					"resolver":  "tar-20",
				}},
				Status: finding.StatusOpen,
			})
			out[len(out)-1].Fill()
		}
	}
	return out
}

func pythonStrExcFindings(f *File, rel, src string) []finding.Finding {
	lines := strings.Split(src, "\n")
	var out []finding.Finding
	for _, m := range pyLabelsStrRE.FindAllStringSubmatchIndex(src, -1) {
		name := src[m[2]:m[3]]
		typ, ok := f.Params[name]
		if !ok || !isExceptionType(typ) {
			continue
		}
		line := strings.Count(src[:m[0]], "\n") + 1
		matched := strings.TrimSpace(lines[line-1])
		if i := strings.Index(matched, "#"); i >= 0 {
			matched = strings.TrimSpace(matched[:i])
		}
		out = append(out, finding.Finding{
			Rule:       ruleUnbounded,
			Language:   "python",
			Severity:   "warning",
			Confidence: "high",
			Location: finding.Location{
				File:   rel,
				Line:   line,
				Symbol: "str(" + name + ")",
			},
			Message: "Metric label value str(" + name + ") is unbounded.",
			Note:    noteStrExc,
			Evidence: finding.Evidence{Static: map[string]any{
				"matched":  matched,
				"param":    name,
				"type":     typ,
				"resolver": "tar-20",
			}},
			Status: finding.StatusOpen,
		})
		out[len(out)-1].Fill()
	}
	return out
}

func isExceptionType(t string) bool {
	switch t {
	case "Exception", "BaseException":
		return true
	default:
		return false
	}
}
