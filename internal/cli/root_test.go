package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSettingsPrecedence(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "log-level: warn\n")

	tests := []struct {
		name string
		env  string
		args []string
		want string
	}{
		{name: "default", want: "info"},
		{name: "config file beats default", args: []string{"--config", filepath.Join(dir, "tarsier.yaml")}, want: "warn"},
		{name: "env beats config file", env: "error", args: []string{"--config", filepath.Join(dir, "tarsier.yaml")}, want: "error"},
		{name: "flag beats env", env: "error", args: []string{"--log-level", "debug"}, want: "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				t.Setenv(envPrefix+"_LOG_LEVEL", tt.env)
			}
			root := NewRootCommand()
			root.SetOut(&bytes.Buffer{})
			root.SetArgs(append(tt.args, "version"))
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if got, _ := root.PersistentFlags().GetString("log-level"); got != tt.want {
				t.Fatalf("log-level = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInvalidLogLevelIsRejected(t *testing.T) {
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--log-level", "loud", "version"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "invalid --log-level") {
		t.Fatalf("want invalid log level error, got %v", err)
	}
}

func TestMissingConfigFileIsAnError(t *testing.T) {
	root := NewRootCommand()
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"--config", filepath.Join(t.TempDir(), "nope.yaml"), "version"})
	if err := root.Execute(); err == nil {
		t.Fatal("an explicitly named config file that does not exist must fail loudly")
	}
}

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "tarsier.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
