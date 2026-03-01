package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"langsmith-fetch-go/internal/core/threads"
)

func TestRunThread_RequiresProjectID(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runThread([]string{"--thread-id", "thread-123"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--project-id is required") {
		t.Fatalf("runThread() error = %v, want project-id error", err)
	}
}

func TestRunThread_RequiresThreadID(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runThread([]string{"--project-id", "project-123"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "--thread-id is required") {
		t.Fatalf("runThread() error = %v, want thread-id error", err)
	}
}

func TestRunThread_ParsesArgsAndCallsService(t *testing.T) {
	fake := &fakeThreadGetter{
		messages: []threads.Message{
			[]byte(`{"role":"user","content":"hello"}`),
		},
	}
	restore := swapThreadGetter(t, fake)
	defer restore()

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runThread([]string{
		"--project-id", "project-123",
		"--thread-id", "thread-123",
		"--format", "json",
	}, &out, &errOut)
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

func TestRunThread_ServiceError(t *testing.T) {
	fake := &fakeThreadGetter{err: errors.New("boom")}
	restore := swapThreadGetter(t, fake)
	defer restore()

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runThread([]string{
		"--project-id", "project-123",
		"--thread-id", "thread-123",
	}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "fetch thread") {
		t.Fatalf("runThread() error = %v, want wrapped fetch error", err)
	}
}

func TestRunThread_PrettyOutput(t *testing.T) {
	fake := &fakeThreadGetter{
		messages: []threads.Message{
			[]byte(`{"role":"assistant","content":"hi"}`),
		},
	}
	restore := swapThreadGetter(t, fake)
	defer restore()

	var out bytes.Buffer
	var errOut bytes.Buffer

	err := runThread([]string{
		"--project-id", "project-123",
		"--thread-id", "thread-123",
		"--format", "pretty",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("runThread() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "[1] {\"role\":\"assistant\"") {
		t.Fatalf("stdout = %q, want pretty message output", got)
	}
}

type fakeThreadGetter struct {
	params   threads.GetParams
	messages []threads.Message
	err      error
}

func (f *fakeThreadGetter) GetMessages(_ context.Context, params threads.GetParams) ([]threads.Message, error) {
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return f.messages, nil
}

func swapThreadGetter(t *testing.T, g threadGetter) func() {
	t.Helper()
	orig := newThreadGetter
	newThreadGetter = func() (threadGetter, error) {
		return g, nil
	}
	return func() { newThreadGetter = orig }
}
