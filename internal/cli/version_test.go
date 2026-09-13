package cli

import (
	"bytes"
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
