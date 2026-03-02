// threads_test.go covers threads command parsing, wiring, and output.
package cmd

import (
	"bytes"
	"context"
	"errors"
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
