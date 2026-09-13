package cli

import (
	"fmt"
	"strings"

	"github.com/Pieczasz/tarsier/internal/finding"
)

const (
	failOnNone    = "none"
	failOnInfo    = "info"
	failOnWarning = "warning"
	failOnError   = "error"
)

// failOnError is returned after a successful scan when --fail-on is tripped.
type failOnThresholdError struct {
	threshold string
	count     int
}

func (e failOnThresholdError) Error() string {
	return fmt.Sprintf("%d findings at or above %s", e.count, e.threshold)
}

func failOnRank(level string) (int, error) {
	switch level {
	case "", failOnNone:
		return 0, nil
	case "hint":
		return 1, nil
	case failOnInfo:
		return 2, nil
	case failOnWarning:
		return 3, nil
	case failOnError:
		return 4, nil
	default:
		return 0, fmt.Errorf("invalid --fail-on %q: want none, info, warning or error", level)
	}
}

func severityRank(sev string) int {
	switch sev {
	case "hint":
		return 1
	case "info":
		return 2
	case "warning":
		return 3
	case "error":
		return 4
	default:
		return 0
	}
}

// blockingCount is the number of non-suppressed findings at or above threshold.
// Medium-confidence findings never gate --fail-on (TAR-19: ambiguous = advisory).
func blockingCount(findings []finding.Finding, threshold int) int {
	if threshold <= 0 {
		return 0
	}
	n := 0
	for i := range findings {
		if findings[i].Status == finding.StatusSuppressed {
			continue
		}
		if strings.EqualFold(findings[i].Confidence, "medium") {
			continue
		}
		if severityRank(findings[i].Severity) >= threshold {
			n++
		}
	}
	return n
}
