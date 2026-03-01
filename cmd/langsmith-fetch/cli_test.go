package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"langsmith-fetch-go/internal/core/traces"
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

func TestRunTraces_ParsesArgsAndCallsService(t *testing.T) {
	fake := &fakeTracesLister{
		runs: []traces.Summary{
			{ID: "trace-1", Name: "hello", StartTime: "2026-01-01T00:00:00Z"},
		},
	}
	restore := swapTracesLister(t, fake)
	defer restore()

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

	if fake.params.ProjectID != "project-123" || fake.params.Limit != 5 {
		t.Fatalf("params = %+v, want project-123/5", fake.params)
	}
	if got := out.String(); !strings.Contains(got, "trace-1") {
		t.Fatalf("stdout = %q, want JSON trace output", got)
	}
}

func TestRunTraces_ServiceError(t *testing.T) {
	fake := &fakeTracesLister{err: errors.New("boom")}
	restore := swapTracesLister(t, fake)
	defer restore()

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runTraces([]string{"--project-id", "project-123"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "list traces") {
		t.Fatalf("runTraces() error = %v, want wrapped list error", err)
	}
}

func TestRunTraces_PrettyOutput(t *testing.T) {
	fake := &fakeTracesLister{
		runs: []traces.Summary{
			{ID: "trace-1", Name: "hello", StartTime: "2026-01-01T00:00:00Z"},
		},
	}
	restore := swapTracesLister(t, fake)
	defer restore()

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runTraces([]string{
		"--project-id", "project-123",
		"--format", "pretty",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("runTraces() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "trace-1\thello") {
		t.Fatalf("stdout = %q, want pretty trace output", got)
	}
}

type fakeTracesLister struct {
	params traces.ListParams
	runs   []traces.Summary
	err    error
}

func (f *fakeTracesLister) List(_ context.Context, params traces.ListParams) ([]traces.Summary, error) {
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return f.runs, nil
}

func swapTracesLister(t *testing.T, l tracesLister) func() {
	t.Helper()
	orig := newTracesLister
	newTracesLister = func() (tracesLister, error) {
		return l, nil
	}
	return func() { newTracesLister = orig }
}
