package main

import (
	"bytes"
	"os"
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

func TestRunHelp(t *testing.T) {
	var errBuf bytes.Buffer
	if code := run([]string{"--help"}, &errBuf); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errBuf.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var errBuf bytes.Buffer
	code := run([]string{"not-a-real-command"}, &errBuf)
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr %q", code, errBuf.String())
	}
	if !strings.Contains(errBuf.String(), "tarsier:") {
		t.Fatalf("stderr = %q", errBuf.String())
	}
}

func TestMainInvokesRun(t *testing.T) {
	oldArgs := os.Args
	oldExit := exitFunc
	defer func() {
		os.Args = oldArgs
		exitFunc = oldExit
	}()
	var code int
	exitFunc = func(c int) { code = c }
	os.Args = []string{"tarsier", "version"}
	main()
	if code != 0 {
		t.Fatalf("main exit %d", code)
	}
}
