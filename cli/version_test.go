package cli

import (
	"bytes"
	"runtime/debug"
	"strings"
	"testing"
)

func TestVersionRespectsOutputFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "text by default", args: []string{"version"}, want: "tarsier"},
		{name: "json on request", args: []string{"--output", "json", "version"}, want: `"revision"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			root := NewRootCommand()
			root.SetOut(&out)
			root.SetArgs(tt.args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tt.want) {
				t.Fatalf("output = %q, want it to contain %q", out.String(), tt.want)
			}
		})
	}
}

func TestBuildInfoFrom(t *testing.T) {
	t.Parallel()

	if got := buildInfoFrom(nil, false); got.Revision != "unknown" || got.Module != "tarsier" {
		t.Fatalf("missing build info: %+v", got)
	}

	raw := &debug.BuildInfo{
		GoVersion: "go1.22.0",
		Main:      debug.Module{Path: "github.com/Pieczasz/tarsier"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc123"},
			{Key: "vcs.time", Value: "2026-01-02T03:04:05Z"},
			{Key: "vcs.modified", Value: "true"},
			{Key: "other", Value: "ignored"},
		},
	}
	got := buildInfoFrom(raw, true)
	if got.Module != "github.com/Pieczasz/tarsier" || got.Revision != "abc123" ||
		got.Time != "2026-01-02T03:04:05Z" || !got.Dirty || got.Go != "go1.22.0" {
		t.Fatalf("unexpected build info: %+v", got)
	}
	text := formatVersionText(got)
	if !strings.Contains(text, "abc123-dirty") {
		t.Fatalf("dirty text = %q", text)
	}
	clean := formatVersionText(buildInfo{Module: "tarsier", Revision: "deadbeef", Go: "go1.22"})
	if strings.Contains(clean, "-dirty") || !strings.Contains(clean, "deadbeef") {
		t.Fatalf("clean text = %q", clean)
	}
}
