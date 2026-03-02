// threads_test.go covers threads command parsing, wiring, and output.
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
	corethreads "langsmith-fetch-go/internal/core/threads"
)

type fakeThreadsLister struct {
	params  corethreads.ListParams
	threads []corethreads.ThreadData
	err     error
}

func (f *fakeThreadsLister) List(_ context.Context, params corethreads.ListParams) ([]corethreads.ThreadData, error) {
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return f.threads, nil
}

func TestRunThreads_RequiresProjectResolution(t *testing.T) {
	t.Parallel()

	err := runThreads(nil, &bytes.Buffer{}, &bytes.Buffer{}, Deps{}, config.Values{APIKey: "test"})
	if err == nil {
		t.Fatal("runThreads() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "--project-id is required") {
		t.Fatalf("runThreads() error = %v, want project id error", err)
	}
}

func TestRunThreads_ParsesArgsAndCallsService(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadsLister{
		threads: []corethreads.ThreadData{
			{
				ThreadID: "thread-1",
				Messages: []corethreads.Message{[]byte(`{"role":"user","content":"hello"}`)},
			},
		},
	}

	var out bytes.Buffer
	err := runThreads(
		[]string{"--project-id", "project-123", "--limit", "5", "--format", "json"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewThreadsLister: func(config.Values) (threadsLister, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runThreads() error = %v", err)
	}
	if fake.params.ProjectID != "project-123" || fake.params.Limit != 5 {
		t.Fatalf("params = %+v, want project-123/5", fake.params)
	}
	if got := out.String(); !strings.Contains(got, "\"thread_id\": \"thread-1\"") {
		t.Fatalf("stdout = %q, want JSON threads output", got)
	}
}

func TestRunThreads_UsesConfigProjectUUID(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadsLister{threads: []corethreads.ThreadData{}}
	err := runThreads(
		[]string{"--format", "json"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewThreadsLister: func(config.Values) (threadsLister, error) { return fake, nil },
		},
		config.Values{APIKey: "test", ProjectUUID: "cfg-project"},
	)
	if err != nil {
		t.Fatalf("runThreads() error = %v", err)
	}
	if fake.params.ProjectID != "cfg-project" {
		t.Fatalf("ProjectID = %q, want %q", fake.params.ProjectID, "cfg-project")
	}
}

func TestRunThreads_InitializeError(t *testing.T) {
	t.Parallel()

	err := runThreads(
		[]string{"--project-id", "project-123"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewThreadsLister: func(config.Values) (threadsLister, error) {
				return nil, errors.New("boom")
			},
		},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "initialize threads service") {
		t.Fatalf("runThreads() error = %v, want initialize error", err)
	}
}

func TestRunThreads_ServiceError(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadsLister{err: errors.New("boom")}
	err := runThreads(
		[]string{"--project-id", "project-123"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewThreadsLister: func(config.Values) (threadsLister, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "list threads") {
		t.Fatalf("runThreads() error = %v, want list threads error", err)
	}
}

func TestRunThreads_PrettyOutput(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadsLister{
		threads: []corethreads.ThreadData{
			{
				ThreadID: "thread-1",
				Messages: []corethreads.Message{[]byte(`{"role":"assistant","content":"hi"}`)},
			},
		},
	}

	var out bytes.Buffer
	err := runThreads(
		[]string{"--project-id", "project-123", "--format", "pretty"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewThreadsLister: func(config.Values) (threadsLister, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runThreads() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Thread: thread-1") || !strings.Contains(got, "[1] {\"role\":\"assistant\"") {
		t.Fatalf("stdout = %q, want pretty threads output", got)
	}
}

func TestRunThreads_WritesSingleFile(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadsLister{
		threads: []corethreads.ThreadData{
			{ThreadID: "thread-1", Messages: []corethreads.Message{[]byte(`{"role":"user","content":"hi"}`)}},
		},
	}
	outFile := filepath.Join(t.TempDir(), "threads.json")
	err := runThreads(
		[]string{"--project-id", "project-123", "--file", outFile},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{NewThreadsLister: func(config.Values) (threadsLister, error) { return fake, nil }},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runThreads() error = %v", err)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "thread-1") {
		t.Fatalf("file = %q, want thread content", string(data))
	}
}

func TestRunThreads_WritesDirectoryFiles(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadsLister{
		threads: []corethreads.ThreadData{
			{ThreadID: "thread-1", Messages: []corethreads.Message{[]byte(`{"role":"assistant","content":"ok"}`)}},
		},
	}
	dir := t.TempDir()
	err := runThreads(
		[]string{
			"--project-id", "project-123",
			"--dir", dir,
			"--filename-pattern", "thread_{thread_id}",
		},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{NewThreadsLister: func(config.Values) (threadsLister, error) { return fake, nil }},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runThreads() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("os.ReadDir() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "thread_thread-1.json" {
		t.Fatalf("entries = %+v, want one thread_thread-1.json", entries)
	}
}

func TestRunThreads_FileAndDirMutuallyExclusive(t *testing.T) {
	t.Parallel()

	err := runThreads(
		[]string{"--project-id", "project-123", "--file", "a.json", "--dir", "out"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("runThreads() error = %v, want mutual exclusivity error", err)
	}
}
