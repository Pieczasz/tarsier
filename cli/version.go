package cli

import (
	"encoding/json"
	"fmt"
	"io"
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
			_, err := io.WriteString(cmd.OutOrStdout(), formatVersionText(info))
			return err
		},
	}
}

func formatVersionText(info buildInfo) string {
	dirty := ""
	if info.Dirty {
		dirty = "-dirty"
	}
	return fmt.Sprintf("%s %s%s (%s)\n", info.Module, info.Revision, dirty, info.Go)
}

func readBuildInfo() buildInfo {
	return buildInfoFrom(debug.ReadBuildInfo())
}

func buildInfoFrom(raw *debug.BuildInfo, ok bool) buildInfo {
	info := buildInfo{Module: "tarsier", Revision: "unknown", Go: "unknown"}
	if !ok || raw == nil {
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
