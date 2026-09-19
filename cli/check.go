package cli

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	enggolang "github.com/Pieczasz/tarsier/engine/golang"
	"github.com/Pieczasz/tarsier/engine/pattern"
	"github.com/Pieczasz/tarsier/finding"
	"github.com/Pieczasz/tarsier/rules"
)

func newCheckCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check [path]",
		Short: "Scan with an opt-in policy; exit non-zero only on blocked classes",
		Long: "Scan a repository and exit non-zero only when findings match " +
			"classes listed under policy.block.\n\n" +
			"Advisory by default: without opted-in classes nothing fails the " +
			"build. Blocking is opt-in per class, and only these four are " +
			"allowed:\n\n" +
			"  unbounded-metric-labels\n" +
			"  secrets-in-logs\n" +
			"  missing-propagation\n" +
			"  removed-required-events\n\n" +
			"Expectation layer 3/4 rules cannot be named as blocking entries " +
			"(rejected at parse time).\n\n" +
			"Exit codes:\n" +
			"  0  no findings in opted-in blocking classes (advisory-only OK)\n" +
			"  1  one or more opted-in blocking findings (or invalid flags)\n\n" +
			"--baseline is honoured so adopting check on an old repo does not " +
			"fail the first build.",
		Args: cobra.MaximumNArgs(1),
		RunE: runCheck,
	}
	cmd.Flags().String("policy", "", "path to observability-policy.yaml (required)")
	cmd.Flags().String("rules", "", "ast-grep project config (default: the embedded rule pack)")
	cmd.Flags().String("engine", enginePattern, "analyzer engine: pattern, go or all")
	cmd.Flags().Duration("timeout", 10*time.Minute, "hard limit on a single pattern-tier scan")
	cmd.Flags().String("baseline", "", "report only findings absent from the baseline file")
	cmd.Flags().String("baseline-write", "", "write current findings as a baseline file")
	_ = cmd.MarkFlagRequired("policy")
	return cmd
}

func runCheck(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) == 1 {
		root = args[0]
	}
	policyPath, _ := cmd.Flags().GetString("policy")
	pol, err := LoadPolicy(policyPath)
	if err != nil {
		return err
	}

	engine, _ := cmd.Flags().GetString("engine")
	switch engine {
	case "", enginePattern, engineGo, engineAll:
	default:
		return fmt.Errorf("invalid --engine %q: want pattern, go or all", engine)
	}
	if engine == "" {
		engine = enginePattern
	}

	config, _ := cmd.Flags().GetString("rules")
	if config == "" && engine != engineGo {
		dir, err := os.MkdirTemp("", "tarsier-rules-")
		if err != nil {
			return err
		}
		defer func() { _ = os.RemoveAll(dir) }()
		if config, err = rules.Materialize(dir); err != nil {
			return err
		}
	}

	timeout, _ := cmd.Flags().GetDuration("timeout")
	started := time.Now()
	findings, err := checkScan(cmd, root, engine, config, timeout)
	if err != nil {
		return err
	}

	baseline, _ := cmd.Flags().GetString("baseline")
	baselineWrite, _ := cmd.Flags().GetString("baseline-write")
	findings, suppressed, known, err := applyLifecycle(root, findings, baseline, baselineWrite)
	if err != nil {
		return err
	}
	slog.Info("check complete",
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
	if n := policyBlockingCount(findings, pol.blockedRules()); n > 0 {
		return policyViolationError{count: n}
	}
	return nil
}

func checkScan(cmd *cobra.Command, root, engine, config string, timeout time.Duration) ([]finding.Finding, error) {
	var findings []finding.Finding
	if engine == enginePattern || engine == engineAll {
		got, err := (&pattern.Runner{Config: config, Timeout: timeout}).Scan(cmd.Context(), root)
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
