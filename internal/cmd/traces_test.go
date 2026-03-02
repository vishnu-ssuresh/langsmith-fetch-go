// traces_test.go covers traces command parsing, wiring, and output.
package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestRunTraces_UsesConfigDefaultFormat(t *testing.T) {
	t.Parallel()

	fake := &fakeTracesLister{
		runs: []coretraces.Summary{
			{ID: "trace-1", Name: "hello", StartTime: "2026-01-01T00:00:00Z"},
		},
	}

	var out bytes.Buffer
	err := runTraces(
		[]string{"--project-id", "project-123"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTracesLister: func(config.Values) (tracesLister, error) { return fake, nil },
		},
		config.Values{APIKey: "test", DefaultFormat: "json"},
	)
	if err != nil {
		t.Fatalf("runTraces() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "\"id\": \"trace-1\"") {
		t.Fatalf("stdout = %q, want json output from config default format", got)
	}
}

func TestRunTraces_WritesSingleFile(t *testing.T) {
	t.Parallel()

	fake := &fakeTracesLister{
		runs: []coretraces.Summary{
			{ID: "trace-1", Name: "hello", StartTime: "2026-01-01T00:00:00Z"},
		},
	}
	outFile := filepath.Join(t.TempDir(), "traces.json")
	err := runTraces(
		[]string{"--project-id", "project-123", "--file", outFile},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{NewTracesLister: func(config.Values) (tracesLister, error) { return fake, nil }},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runTraces() error = %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "trace-1") {
		t.Fatalf("file = %q, want trace content", string(data))
	}
}

func TestRunTraces_WritesDirectoryFiles(t *testing.T) {
	t.Parallel()

	fake := &fakeTracesLister{
		runs: []coretraces.Summary{
			{ID: "trace-1", Name: "hello", StartTime: "2026-01-01T00:00:00Z"},
		},
	}
	dir := t.TempDir()
	err := runTraces(
		[]string{
			"--project-id", "project-123",
			"--dir", dir,
			"--filename-pattern", "trace_{index}",
		},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{NewTracesLister: func(config.Values) (tracesLister, error) { return fake, nil }},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runTraces() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("os.ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "trace_1.json" {
		t.Fatalf("entries = %+v, want one trace_1.json", entries)
	}
}

func TestRunTraces_FileAndDirMutuallyExclusive(t *testing.T) {
	t.Parallel()

	err := runTraces(
		[]string{"--project-id", "project-123", "--file", "a.json", "--dir", "out"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("runTraces() error = %v, want mutual exclusivity error", err)
	}
}

func TestRunTraces_UsesConfigProjectUUID(t *testing.T) {
	t.Parallel()

	fake := &fakeTracesLister{runs: []coretraces.Summary{}}
	err := runTraces(
		[]string{"--format", "json"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewTracesLister: func(config.Values) (tracesLister, error) { return fake, nil },
		},
		config.Values{APIKey: "test", ProjectUUID: "cfg-project"},
	)
	if err != nil {
		t.Fatalf("runTraces() error = %v", err)
	}
	if fake.params.ProjectID != "cfg-project" {
		t.Fatalf("ProjectID = %q, want %q", fake.params.ProjectID, "cfg-project")
	}
}

func TestRunTraces_ResolvesProjectName(t *testing.T) {
	t.Parallel()

	fake := &fakeTracesLister{runs: []coretraces.Summary{}}
	project := &fakeProjectResolver{id: "resolved-project-id"}

	err := runTraces(
		[]string{"--format", "json"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewTracesLister:    func(config.Values) (tracesLister, error) { return fake, nil },
			NewProjectResolver: func(config.Values) (projectResolver, error) { return project, nil },
		},
		config.Values{APIKey: "test", ProjectName: "my-project"},
	)
	if err != nil {
		t.Fatalf("runTraces() error = %v", err)
	}
	if project.name != "my-project" {
		t.Fatalf("resolver name = %q, want %q", project.name, "my-project")
	}
	if fake.params.ProjectID != "resolved-project-id" {
		t.Fatalf("ProjectID = %q, want %q", fake.params.ProjectID, "resolved-project-id")
	}
}
