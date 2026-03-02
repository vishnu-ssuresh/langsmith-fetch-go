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
	if got := out.String(); !strings.Contains(got, "[1] {\"role\":\"assistant\"") {
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
	if got := out.String(); !strings.Contains(got, "[1] {\"role\":\"assistant\"") {
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
