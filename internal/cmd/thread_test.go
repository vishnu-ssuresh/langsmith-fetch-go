// thread_test.go covers thread command parsing, wiring, and output.
package cmd

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"langsmith-fetch-go/internal/config"
	coresingle "langsmith-fetch-go/internal/core/single"
)

type fakeThreadGetter struct {
	params   coresingle.ThreadParams
	messages []coresingle.ThreadMessage
	err      error
}

func (f *fakeThreadGetter) GetMessages(_ context.Context, params coresingle.ThreadParams) ([]coresingle.ThreadMessage, error) {
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return f.messages, nil
}

func TestRunThread_RequiresProjectID(t *testing.T) {
	t.Parallel()

	err := runThread(
		[]string{"--thread-id", "thread-123"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{},
		config.Values{APIKey: "test"},
	)
	if err == nil {
		t.Fatal("runThread() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "--project-id is required") {
		t.Fatalf("runThread() error = %v, want project-id error", err)
	}
}

func TestRunThread_RequiresThreadID(t *testing.T) {
	t.Parallel()

	err := runThread(
		[]string{"--project-id", "project-123"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{},
		config.Values{APIKey: "test"},
	)
	if err == nil {
		t.Fatal("runThread() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "--thread-id is required") {
		t.Fatalf("runThread() error = %v, want thread-id error", err)
	}
}

func TestRunThread_ParsesArgsAndCallsService(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadGetter{
		messages: []coresingle.ThreadMessage{
			[]byte(`{"role":"user","content":"hello"}`),
		},
	}

	var out bytes.Buffer
	err := runThread(
		[]string{
			"--project-id", "project-123",
			"--thread-id", "thread-123",
			"--format", "json",
		},
		&out,
		&bytes.Buffer{},
		Deps{
			NewThreadGetter: func(config.Values) (threadGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runThread() error = %v", err)
	}
	if fake.params.ProjectID != "project-123" || fake.params.ThreadID != "thread-123" {
		t.Fatalf("params = %+v, want project-123/thread-123", fake.params)
	}
	if got := out.String(); !strings.Contains(got, "\"role\": \"user\"") {
		t.Fatalf("stdout = %q, want JSON message output", got)
	}
}

func TestRunThread_InitializeError(t *testing.T) {
	t.Parallel()

	err := runThread(
		[]string{"--project-id", "project-123", "--thread-id", "thread-123"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewThreadGetter: func(config.Values) (threadGetter, error) {
				return nil, errors.New("boom")
			},
		},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "initialize threads service") {
		t.Fatalf("runThread() error = %v, want initialize error", err)
	}
}

func TestRunThread_ServiceError(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadGetter{err: errors.New("boom")}
	err := runThread(
		[]string{"--project-id", "project-123", "--thread-id", "thread-123"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewThreadGetter: func(config.Values) (threadGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "fetch thread") {
		t.Fatalf("runThread() error = %v, want fetch error", err)
	}
}

func TestRunThread_PrettyOutput(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadGetter{
		messages: []coresingle.ThreadMessage{
			[]byte(`{"role":"assistant","content":"hi"}`),
		},
	}

	var out bytes.Buffer
	err := runThread(
		[]string{
			"--project-id", "project-123",
			"--thread-id", "thread-123",
			"--format", "pretty",
		},
		&out,
		&bytes.Buffer{},
		Deps{
			NewThreadGetter: func(config.Values) (threadGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runThread() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "[1] {\"role\":\"assistant\"") {
		t.Fatalf("stdout = %q, want pretty message output", got)
	}
}

func TestRunThread_UsesConfigDefaultFormat(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadGetter{
		messages: []coresingle.ThreadMessage{
			[]byte(`{"role":"user","content":"hello"}`),
		},
	}

	var out bytes.Buffer
	err := runThread(
		[]string{"--project-id", "project-123", "--thread-id", "thread-123"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewThreadGetter: func(config.Values) (threadGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test", DefaultFormat: "json"},
	)
	if err != nil {
		t.Fatalf("runThread() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "\"role\": \"user\"") {
		t.Fatalf("stdout = %q, want json output from config default format", got)
	}
}

func TestRunThread_UsesConfigProjectUUID(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadGetter{messages: []coresingle.ThreadMessage{}}
	err := runThread(
		[]string{"--thread-id", "thread-123", "--format", "json"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewThreadGetter: func(config.Values) (threadGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test", ProjectUUID: "cfg-project"},
	)
	if err != nil {
		t.Fatalf("runThread() error = %v", err)
	}
	if fake.params.ProjectID != "cfg-project" {
		t.Fatalf("ProjectID = %q, want %q", fake.params.ProjectID, "cfg-project")
	}
}

func TestRunThread_ResolvesProjectName(t *testing.T) {
	t.Parallel()

	fake := &fakeThreadGetter{messages: []coresingle.ThreadMessage{}}
	project := &fakeProjectResolver{id: "resolved-project-id"}

	err := runThread(
		[]string{"--thread-id", "thread-123", "--format", "json"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewThreadGetter:    func(config.Values) (threadGetter, error) { return fake, nil },
			NewProjectResolver: func(config.Values) (projectResolver, error) { return project, nil },
		},
		config.Values{APIKey: "test", ProjectName: "my-project"},
	)
	if err != nil {
		t.Fatalf("runThread() error = %v", err)
	}
	if project.name != "my-project" {
		t.Fatalf("resolver name = %q, want %q", project.name, "my-project")
	}
	if fake.params.ProjectID != "resolved-project-id" {
		t.Fatalf("ProjectID = %q, want %q", fake.params.ProjectID, "resolved-project-id")
	}
}
