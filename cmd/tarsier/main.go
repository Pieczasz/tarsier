// Command tarsier audits repositories for observability gaps.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Pieczasz/tarsier/cli"
)

// exitFunc is swapped in tests so main can be exercised without killing the process.
var exitFunc = os.Exit

func main() {
	exitFunc(run(os.Args[1:], os.Stderr))
}

func run(args []string, errOut io.Writer) int {
	root := cli.NewRootCommand()
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(errOut, "tarsier:", err)
		return 1
	}
	return 0
}
