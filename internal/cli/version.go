package cli

import (
	"encoding/json"
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

type buildInfo struct {
	Module   string `json:"module"`
	Revision string `json:"revision"`
	Time     string `json:"time,omitempty"`
	Dirty    bool   `json:"dirty"`
	Go       string `json:"go"`
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := readBuildInfo()
			format, _ := cmd.Flags().GetString("output")
			if format == formatJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			dirty := ""
			if info.Dirty {
				dirty = "-dirty"
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s%s (%s)\n",
				info.Module, info.Revision, dirty, info.Go)
			return err
		},
	}
}

func readBuildInfo() buildInfo {
	info := buildInfo{Module: "tarsier", Revision: "unknown", Go: "unknown"}
	raw, ok := debug.ReadBuildInfo()
	if !ok {
		return info
	}
	info.Module, info.Go = raw.Main.Path, raw.GoVersion
	for _, s := range raw.Settings {
		switch s.Key {
		case "vcs.revision":
			info.Revision = s.Value
		case "vcs.time":
			info.Time = s.Value
		case "vcs.modified":
			info.Dirty = s.Value == "true"
		}
	}
	return info
}
