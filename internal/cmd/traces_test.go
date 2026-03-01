package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"langsmith-fetch-go/internal/config"
	coretraces "langsmith-fetch-go/internal/core/traces"
)

type fakeTracesLister struct {
	params coretraces.ListParams
	runs   []coretraces.Summary
	err    error
}

func (f *fakeTracesLister) List(_ context.Context, params coretraces.ListParams) ([]coretraces.Summary, error) {
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return f.runs, nil
}

func TestRunTraces_RequiresProjectID(t *testing.T) {
	t.Parallel()

	err := runTraces(nil, &bytes.Buffer{}, &bytes.Buffer{}, Deps{}, config.Values{APIKey: "test"})
	if err == nil {
		t.Fatal("runTraces() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "--project-id is required") {
		t.Fatalf("runTraces() error = %v, want project-id error", err)
	}
}

func TestRunTraces_ParsesArgsAndCallsService(t *testing.T) {
	t.Parallel()

	fake := &fakeTracesLister{
		runs: []coretraces.Summary{
			{ID: "trace-1", Name: "hello", StartTime: "2026-01-01T00:00:00Z"},
		},
	}

	var out bytes.Buffer
	err := runTraces(
		[]string{"--project-id", "project-123", "--limit", "5", "--format", "json"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTracesLister: func(config.Values) (tracesLister, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
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

func TestRunTraces_InitializeError(t *testing.T) {
	t.Parallel()

	err := runTraces(
		[]string{"--project-id", "project-123"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewTracesLister: func(config.Values) (tracesLister, error) {
				return nil, errors.New("boom")
			},
		},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "initialize traces service") {
		t.Fatalf("runTraces() error = %v, want initialize error", err)
	}
}

func TestRunTraces_ServiceError(t *testing.T) {
	t.Parallel()

	fake := &fakeTracesLister{err: errors.New("boom")}
	err := runTraces(
		[]string{"--project-id", "project-123"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewTracesLister: func(config.Values) (tracesLister, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "list traces") {
		t.Fatalf("runTraces() error = %v, want list traces error", err)
	}
}

func TestRunTraces_PrettyOutput(t *testing.T) {
	t.Parallel()

	fake := &fakeTracesLister{
		runs: []coretraces.Summary{
			{ID: "trace-1", Name: "hello", StartTime: "2026-01-01T00:00:00Z"},
		},
	}

	var out bytes.Buffer
	err := runTraces(
		[]string{"--project-id", "project-123", "--format", "pretty"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTracesLister: func(config.Values) (tracesLister, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runTraces() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "trace-1\thello") {
		t.Fatalf("stdout = %q, want pretty trace output", got)
	}
}
