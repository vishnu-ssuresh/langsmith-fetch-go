// trace_test.go covers trace command parsing, wiring, and output.
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
	coresingle "langsmith-fetch-go/internal/core/single"
	langsmithfeedback "langsmith-fetch-go/internal/langsmith/feedback"
)

type fakeTraceGetter struct {
	params   coresingle.TraceParams
	messages []coresingle.Message
	err      error
}

func (f *fakeTraceGetter) GetMessages(_ context.Context, params coresingle.TraceParams) ([]coresingle.Message, error) {
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return f.messages, nil
}

func (f *fakeTraceGetter) GetRun(_ context.Context, params coresingle.TraceParams) (coresingle.Run, error) {
	f.params = params
	if f.err != nil {
		return coresingle.Run{}, f.err
	}
	return coresingle.Run{
		ID:       params.TraceID,
		Messages: f.messages,
	}, nil
}

type fakeTraceFeedbackAccessor struct {
	items []langsmithfeedback.Item
	err   error
}

func (f *fakeTraceFeedbackAccessor) ListByRuns(_ context.Context, _ langsmithfeedback.ListParams) ([]langsmithfeedback.Item, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.items, nil
}

func TestRunTrace_RequiresTraceID(t *testing.T) {
	t.Parallel()

	err := runTrace(nil, &bytes.Buffer{}, &bytes.Buffer{}, Deps{}, config.Values{APIKey: "test"})
	if err == nil {
		t.Fatal("runTrace() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "--trace-id is required") {
		t.Fatalf("runTrace() error = %v, want trace-id error", err)
	}
}

func TestRunTrace_ParsesArgsAndCallsService(t *testing.T) {
	t.Parallel()

	fake := &fakeTraceGetter{
		messages: []coresingle.Message{
			[]byte(`{"role":"user","content":"hello"}`),
		},
	}

	var out bytes.Buffer
	err := runTrace(
		[]string{"--trace-id", "trace-123", "--format", "json"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runTrace() error = %v", err)
	}
	if fake.params.TraceID != "trace-123" {
		t.Fatalf("TraceID = %q, want %q", fake.params.TraceID, "trace-123")
	}
	if got := out.String(); !strings.Contains(got, "\"role\": \"user\"") {
		t.Fatalf("stdout = %q, want JSON message output", got)
	}
}

func TestRunTrace_SupportsPositionalTraceID(t *testing.T) {
	t.Parallel()

	fake := &fakeTraceGetter{
		messages: []coresingle.Message{
			[]byte(`{"role":"user","content":"hello"}`),
		},
	}

	var out bytes.Buffer
	err := runTrace(
		[]string{"trace-123", "--format", "json"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runTrace() error = %v", err)
	}
	if fake.params.TraceID != "trace-123" {
		t.Fatalf("TraceID = %q, want %q", fake.params.TraceID, "trace-123")
	}
}

func TestRunTrace_RejectsExtraPositionalArgs(t *testing.T) {
	t.Parallel()

	err := runTrace(
		[]string{"trace-123", "extra"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return &fakeTraceGetter{}, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Fatalf("runTrace() error = %v, want positional argument error", err)
	}
}

func TestRunTrace_InitializeError(t *testing.T) {
	t.Parallel()

	err := runTrace(
		[]string{"--trace-id", "trace-123"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) {
				return nil, errors.New("boom")
			},
		},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "initialize trace service") {
		t.Fatalf("runTrace() error = %v, want initialize error", err)
	}
}

func TestRunTrace_ServiceError(t *testing.T) {
	t.Parallel()

	fake := &fakeTraceGetter{err: errors.New("boom")}
	err := runTrace(
		[]string{"--trace-id", "trace-123"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "fetch trace") {
		t.Fatalf("runTrace() error = %v, want fetch error", err)
	}
}

func TestRunTrace_PrettyOutput(t *testing.T) {
	t.Parallel()

	fake := &fakeTraceGetter{
		messages: []coresingle.Message{
			[]byte(`{"role":"assistant","content":"hi"}`),
		},
	}

	var out bytes.Buffer
	err := runTrace(
		[]string{"--trace-id", "trace-123", "--format", "pretty"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runTrace() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Trace Messages (1)") ||
		!strings.Contains(got, "1. assistant") {
		t.Fatalf("stdout = %q, want pretty message output", got)
	}
}

func TestRunTrace_UsesConfigDefaultFormat(t *testing.T) {
	t.Parallel()

	fake := &fakeTraceGetter{
		messages: []coresingle.Message{
			[]byte(`{"role":"user","content":"hello"}`),
		},
	}

	var out bytes.Buffer
	err := runTrace(
		[]string{"--trace-id", "trace-123"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test", DefaultFormat: "json"},
	)
	if err != nil {
		t.Fatalf("runTrace() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "\"role\": \"user\"") {
		t.Fatalf("stdout = %q, want json output from config default format", got)
	}
}

func TestRunTrace_FlagFormatOverridesConfigDefault(t *testing.T) {
	t.Parallel()

	fake := &fakeTraceGetter{
		messages: []coresingle.Message{
			[]byte(`{"role":"assistant","content":"hi"}`),
		},
	}

	var out bytes.Buffer
	err := runTrace(
		[]string{"--trace-id", "trace-123", "--format", "pretty"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test", DefaultFormat: "json"},
	)
	if err != nil {
		t.Fatalf("runTrace() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "Trace Messages (1)") ||
		!strings.Contains(got, "1. assistant") {
		t.Fatalf("stdout = %q, want pretty output from explicit --format", got)
	}
}

func TestRunTrace_WritesSingleFile(t *testing.T) {
	t.Parallel()

	fake := &fakeTraceGetter{
		messages: []coresingle.Message{
			[]byte(`{"role":"user","content":"hello"}`),
		},
	}

	outFile := filepath.Join(t.TempDir(), "trace.json")
	var out bytes.Buffer
	err := runTrace(
		[]string{"--trace-id", "trace-123", "--format", "json", "--file", outFile},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return fake, nil },
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runTrace() error = %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when --file is used", out.String())
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "\"role\": \"user\"") {
		t.Fatalf("file = %q, want JSON trace output", string(data))
	}
}

func TestRunTrace_IncludeMetadataOutput(t *testing.T) {
	t.Parallel()

	fake := &fakeTraceGetter{
		messages: []coresingle.Message{
			[]byte(`{"role":"assistant","content":"hello"}`),
		},
	}

	var out bytes.Buffer
	err := runTrace(
		[]string{"--trace-id", "trace-123", "--include-metadata", "--format", "json"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return fake, nil },
			NewFeedbackAccessor: func(config.Values) (traceFeedbackAccessor, error) {
				return &fakeTraceFeedbackAccessor{}, nil
			},
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runTrace() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "\"trace_id\": \"trace-123\"") {
		t.Fatalf("stdout = %q, want trace_id", got)
	}
	if !strings.Contains(got, "\"metadata\"") {
		t.Fatalf("stdout = %q, want metadata section", got)
	}
}

func TestRunTrace_IncludeFeedbackOutput(t *testing.T) {
	t.Parallel()

	fake := &fakeTraceGetter{
		messages: []coresingle.Message{
			[]byte(`{"role":"assistant","content":"hello"}`),
		},
	}
	fb := &fakeTraceFeedbackAccessor{
		items: []langsmithfeedback.Item{
			{ID: "fb-1", RunID: "trace-123", Key: "correctness"},
		},
	}

	var out bytes.Buffer
	err := runTrace(
		[]string{"--trace-id", "trace-123", "--include-feedback", "--format", "json"},
		&out,
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return fake, nil },
			NewFeedbackAccessor: func(config.Values) (traceFeedbackAccessor, error) {
				return fb, nil
			},
		},
		config.Values{APIKey: "test"},
	)
	if err != nil {
		t.Fatalf("runTrace() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "\"feedback\"") || !strings.Contains(got, "\"fb-1\"") {
		t.Fatalf("stdout = %q, want feedback payload", got)
	}
}

func TestRunTrace_IncludeFeedbackInitError(t *testing.T) {
	t.Parallel()

	fake := &fakeTraceGetter{
		messages: []coresingle.Message{
			[]byte(`{"role":"assistant","content":"hello"}`),
		},
	}

	err := runTrace(
		[]string{"--trace-id", "trace-123", "--include-feedback", "--format", "json"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{
			NewTraceGetter: func(config.Values) (traceGetter, error) { return fake, nil },
			NewFeedbackAccessor: func(config.Values) (traceFeedbackAccessor, error) {
				return nil, errors.New("boom")
			},
		},
		config.Values{APIKey: "test"},
	)
	if err == nil || !strings.Contains(err.Error(), "initialize feedback accessor") {
		t.Fatalf("runTrace() error = %v, want feedback init error", err)
	}
}
