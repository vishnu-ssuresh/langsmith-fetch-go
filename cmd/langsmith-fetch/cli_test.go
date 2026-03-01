package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestExecute_ShowsUsageWhenNoArgs(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := execute(nil, &out, &errOut)
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Fatalf("stdout = %q, want usage text", out.String())
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := execute([]string{"unknown"}, &out, &errOut)
	if err == nil {
		t.Fatal("execute() error = nil, want non-nil")
	}
}
