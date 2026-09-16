package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunVersion(t *testing.T) {
	var errBuf bytes.Buffer
	if code := run([]string{"version"}, &errBuf); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errBuf.String())
	}
}

func TestRunReportsErrors(t *testing.T) {
	var errBuf bytes.Buffer
	code := run([]string{"--log-level", "loud", "version"}, &errBuf)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errBuf.String(), "tarsier:") {
		t.Fatalf("stderr = %q", errBuf.String())
	}
}
