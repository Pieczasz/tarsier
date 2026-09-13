package cli

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Pieczasz/tarsier/internal/engine/pattern"
	"github.com/Pieczasz/tarsier/internal/finding"
	"github.com/Pieczasz/tarsier/internal/rules"
)

// Free CLI slice for the cardinality-check skill (TAR-23).
func newCheckCardinalityCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-cardinality [path]",
		Short: "Scan only high-cardinality / unbounded label rules",
		Long: "Free CLI slice used by the cardinality-check skill.\n\n" +
			"Runs the pattern engine and keeps only metrics/high-cardinality-label " +
			"and metrics/unbounded-label-value findings. Re-run after edits to verify.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			dir, err := os.MkdirTemp("", "tarsier-rules-")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(dir) }()
			config, err := rules.Materialize(dir)
			if err != nil {
				return err
			}
			timeout, _ := cmd.Flags().GetDuration("timeout")
			started := time.Now()
			all, err := (&pattern.Runner{Config: config, Timeout: timeout}).Scan(cmd.Context(), root)
			if err != nil {
				return err
			}
			var findings []finding.Finding
			for i := range all {
				if isCardinalityRule(all[i].Rule) {
					findings = append(findings, all[i])
				}
			}
			slog.Info("check-cardinality complete",
				"root", root,
				"findings", len(findings),
				"duration_ms", time.Since(started).Milliseconds())
			format, _ := cmd.Flags().GetString("output")
			if err := render(cmd.OutOrStdout(), findings, format, root); err != nil {
				return err
			}
			failOn, _ := cmd.Flags().GetString("fail-on")
			threshold, err := failOnRank(failOn)
			if err != nil {
				return err
			}
			if n := blockingCount(findings, threshold); n > 0 {
				return failOnThresholdError{threshold: failOn, count: n}
			}
			if len(findings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "ok: no cardinality findings")
			}
			return nil
		},
	}
	cmd.Flags().Duration("timeout", 10*time.Minute, "hard limit on a single scan")
	cmd.Flags().String("fail-on", failOnNone, "exit 1 if a non-suppressed finding is at or above this severity")
	return cmd
}

func isCardinalityRule(rule string) bool {
	return strings.HasPrefix(rule, "metrics/high-cardinality-label") ||
		strings.HasPrefix(rule, "metrics/unbounded-label-value")
}
