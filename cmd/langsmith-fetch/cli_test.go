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

func TestRunTraces_RequiresProjectID(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runTraces(nil, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--project-id is required") {
		t.Fatalf("runTraces() error = %v, want project-id error", err)
	}
}

func TestRunTraces_ParsesArgs(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runTraces([]string{
		"--project-id", "project-123",
		"--limit", "5",
		"--format", "json",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("runTraces() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "project-123") {
		t.Fatalf("stdout = %q, want parsed project id", got)
	}
}
