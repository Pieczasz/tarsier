// Command tarsier audits repositories for observability gaps.
package main

import (
	"fmt"
	"os"

	"github.com/Pieczasz/tarsier/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "tarsier:", err)
		os.Exit(1)
	}
}
