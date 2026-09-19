package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	enggolang "github.com/Pieczasz/tarsier/engine/golang"
	"github.com/Pieczasz/tarsier/engine/pattern"
	"github.com/Pieczasz/tarsier/finding"
	"github.com/Pieczasz/tarsier/report"
	"github.com/Pieczasz/tarsier/rules"
)

const (
	enginePattern = "pattern"
	engineGo      = "go"
	engineAll     = "all"
)

func newScanCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scan [path]",
		Short: "Scan a repository for observability gaps",
		Long: "Scan a repository for observability gaps.\n\n" +
			"Vendored, generated, and test-marked paths are excluded " +
			"(vendor, node_modules, dist, build, tools, integration, " +
			"*_test.go, __tests__, testdata, examples, etc.). " +
			"Excluded matches are counted at debug log level.\n\n" +
			"Findings carrying a tarsier:ignore comment (with a reason) are " +
			"reported as suppressed; --baseline limits output to new findings.\n\n" +
			"--engine selects analyzers: pattern (default, ast-grep), go " +
			"(go/analysis deep tier), or all. Medium-confidence findings " +
			"never trip --fail-on.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}

			engine, err := normalizeEngine(flagString(cmd, "engine"))
			if err != nil {
				return err
			}
			config := flagString(cmd, "rules")
			cleanup, err := ensurePatternRules(engine, &config)
			if err != nil {
				return err
			}
			if cleanup != nil {
				defer cleanup()
			}
			failOn, _ := cmd.Flags().GetString("fail-on")
			threshold, err := failOnRank(failOn)
			if err != nil {
				return err
			}

			timeout, _ := cmd.Flags().GetDuration("timeout")
			started := time.Now()
			findings, err := runEngines(cmd.Context(), root, engine, config, timeout)
			if err != nil {
				return err
			}

			baseline, _ := cmd.Flags().GetString("baseline")
			baselineWrite, _ := cmd.Flags().GetString("baseline-write")
			findings, suppressed, known, err := applyLifecycle(root, findings, baseline, baselineWrite)
			if err != nil {
				return err
			}
			slog.Info("scan complete",
				"root", root,
				"engine", engine,
				"findings", len(findings),
				"suppressed", suppressed,
				"known", known,
				"duration_ms", time.Since(started).Milliseconds())

			format, _ := cmd.Flags().GetString("output")
			if err := render(cmd.OutOrStdout(), findings, format, root); err != nil {
				return err
			}
			if n := blockingCount(findings, threshold); n > 0 {
				return failOnThresholdError{threshold: failOn, count: n}
			}
			return nil
		},
	}
	cmd.Flags().String("rules", "", "ast-grep project config (default: the embedded rule pack)")
	cmd.Flags().String("engine", enginePattern, "analyzer engine: pattern, go or all")
	cmd.Flags().Duration("timeout", 10*time.Minute, "hard limit on a single pattern-tier scan")
	cmd.Flags().String("baseline", "", "report only findings absent from the baseline file")
	cmd.Flags().String("baseline-write", "", "write current findings as a baseline file")
	cmd.Flags().String("fail-on", failOnNone, "exit 1 if a non-suppressed finding is at or above this severity: none, info, warning, error")
	return cmd
}

func normalizeEngine(engine string) (string, error) {
	switch engine {
	case "", enginePattern, engineGo, engineAll:
	default:
		return "", fmt.Errorf("invalid --engine %q: want pattern, go or all", engine)
	}
	if engine == "" {
		return enginePattern, nil
	}
	return engine, nil
}

func flagString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)
	return v
}

// ensurePatternRules materializes the embedded pack when pattern tier runs
// without --rules. cleanup removes the temp dir; nil means nothing to do.
func ensurePatternRules(engine string, config *string) (cleanup func(), err error) {
	if *config != "" || engine == engineGo {
		return nil, nil
	}
	dir, err := os.MkdirTemp("", "tarsier-rules-")
	if err != nil {
		return nil, err
	}
	path, err := rules.Materialize(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	*config = path
	return func() { _ = os.RemoveAll(dir) }, nil
}

func runEngines(ctx context.Context, root, engine, config string, timeout time.Duration) ([]finding.Finding, error) {
	var findings []finding.Finding
	if engine == enginePattern || engine == engineAll {
		got, err := (&pattern.Runner{Config: config, Timeout: timeout}).Scan(ctx, root)
		if err != nil {
			return nil, err
		}
		findings = append(findings, got...)
	}
	if engine == engineGo || engine == engineAll {
		got, err := (&enggolang.Runner{}).Scan(root)
		if err != nil {
			return nil, err
		}
		findings = append(findings, got...)
	}
	return findings, nil
}

// sourcePath resolves a finding's scan-relative path against the scan root.
func sourcePath(root, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	if info, err := os.Stat(root); err == nil && !info.IsDir() {
		if rel == root || filepath.Base(rel) == filepath.Base(root) {
			return root
		}
		return filepath.Join(filepath.Dir(root), rel)
	}
	return filepath.Join(root, rel)
}

// loadSuppressions reads each implicated source file once. An unreadable
// file only warns: a deleted file must not fail the scan that found it.
func loadSuppressions(root string, findings []finding.Finding) []finding.Suppression {
	seen := map[string]bool{}
	var out []finding.Suppression
	for i := range findings {
		path := sourcePath(root, findings[i].Location.File)
		if seen[path] {
			continue
		}
		seen[path] = true
		src, err := os.ReadFile(path) //nolint:gosec // G304: paths come from the user's own scanned tree, not remote input
		if err != nil {
			slog.Warn("cannot read source for suppression check", "path", path, "detail", err.Error())
			continue
		}
		out = append(out, finding.ParseSuppressions(findings[i].Location.File, src)...)
	}
	return out
}

// applyLifecycle suppresses, optionally records a baseline, and optionally
// filters to new findings. Suppression runs first so a suppressed finding
// never needs baselining; suppressed findings stay in the output, marked.
// The baseline is loaded before a same-path write so
// --baseline X --baseline-write X reports new findings against the previous
// set, then records the current one.
//
// Output is rebuilt from the partitioned copies ApplySuppressions returns,
// never by re-reading Status on the input slice.
func applyLifecycle(root string, findings []finding.Finding, baselineFile, baselineWriteFile string) (out []finding.Finding, suppressed, known int, err error) {
	sup := loadSuppressions(root, findings)
	kept, supp := finding.ApplySuppressions(findings, sup)
	suppressed = len(supp)
	knownSet := map[string]bool{}
	if baselineFile != "" {
		knownSet, err = finding.LoadBaseline(baselineFile)
		if err != nil {
			return nil, 0, 0, err
		}
	}
	if baselineWriteFile != "" {
		if err := finding.WriteBaseline(baselineWriteFile, kept); err != nil {
			return nil, 0, 0, err
		}
	}
	fresh, knownList := finding.PartitionNew(kept, knownSet)
	known = len(knownList)
	return mergeLifecycle(findings, fresh, supp), suppressed, known, nil
}

// mergeLifecycle emits fresh open findings and all suppressed findings in the
// original scan order. Known open findings are dropped.
func mergeLifecycle(original, fresh, suppressed []finding.Finding) []finding.Finding {
	emit := make(map[string]finding.Finding, len(fresh)+len(suppressed))
	for i := range fresh {
		emit[fresh[i].Fingerprint] = fresh[i]
	}
	for i := range suppressed {
		emit[suppressed[i].Fingerprint] = suppressed[i]
	}
	out := make([]finding.Finding, 0, len(emit))
	for i := range original {
		if f, ok := emit[original[i].Fingerprint]; ok {
			out = append(out, f)
		}
	}
	return out
}

func render(w io.Writer, findings []finding.Finding, format, root string) error {
	if format == formatJSON {
		if findings == nil {
			findings = []finding.Finding{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(findings)
	}
	if format == formatHTML {
		s := report.Build(root, findings, time.Now())
		return report.WriteHTML(w, &s)
	}
	if format != formatText && format != "" {
		return fmt.Errorf("invalid --output %q: want text, json or html", format)
	}

	for i := range findings {
		f := &findings[i]
		suffix := ""
		if f.Status == finding.StatusSuppressed {
			suffix = " [suppressed]"
		}
		if _, err := fmt.Fprintf(w, "%s:%d: %s: %s%s\n",
			f.Location.File, f.Location.Line, f.Severity, f.Message, suffix); err != nil {
			return err
		}
		if f.Note == "" {
			continue
		}
		for line := range strings.SplitSeq(strings.TrimSpace(f.Note), "\n") {
			if _, err := fmt.Fprintf(w, "    %s\n", line); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(w, "%d findings\n", len(findings))
	return err
}
