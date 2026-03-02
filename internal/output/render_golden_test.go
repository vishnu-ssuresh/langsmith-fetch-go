// render_golden_test.go validates exact output snapshots for all render modes.
package output

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	coresingle "langsmith-fetch-go/internal/core/single"
	corethreads "langsmith-fetch-go/internal/core/threads"
	coretraces "langsmith-fetch-go/internal/core/traces"
)

func TestWriteTraceMessages_Golden(t *testing.T) {
	t.Parallel()

	messages := []coresingle.Message{
		[]byte(`{"role":"user","content":"hello"}`),
		[]byte(`{"role":"assistant","content":"world"}`),
	}

	assertGoldenOutput(t, "trace_messages_pretty.golden", func(buf *bytes.Buffer) error {
		return WriteTraceMessages(buf, "pretty", messages)
	})
	assertGoldenOutput(t, "trace_messages_json.golden", func(buf *bytes.Buffer) error {
		return WriteTraceMessages(buf, "json", messages)
	})
	assertGoldenOutput(t, "trace_messages_raw.golden", func(buf *bytes.Buffer) error {
		return WriteTraceMessages(buf, "raw", messages)
	})
}

func TestWriteThreadMessages_Golden(t *testing.T) {
	t.Parallel()

	messages := []coresingle.ThreadMessage{
		[]byte(`{"role":"user","content":"hello"}`),
		[]byte(`{"role":"assistant","content":"world"}`),
	}

	assertGoldenOutput(t, "thread_messages_pretty.golden", func(buf *bytes.Buffer) error {
		return WriteThreadMessages(buf, "pretty", messages)
	})
	assertGoldenOutput(t, "thread_messages_json.golden", func(buf *bytes.Buffer) error {
		return WriteThreadMessages(buf, "json", messages)
	})
	assertGoldenOutput(t, "thread_messages_raw.golden", func(buf *bytes.Buffer) error {
		return WriteThreadMessages(buf, "raw", messages)
	})
}

func TestWriteTraceSummaries_Golden(t *testing.T) {
	t.Parallel()

	runs := []coretraces.Summary{
		{ID: "trace-1", Name: "hello", StartTime: "2026-01-01T00:00:00Z"},
	}

	assertGoldenOutput(t, "trace_summaries_pretty.golden", func(buf *bytes.Buffer) error {
		return WriteTraceSummaries(buf, "pretty", runs)
	})
	assertGoldenOutput(t, "trace_summaries_json.golden", func(buf *bytes.Buffer) error {
		return WriteTraceSummaries(buf, "json", runs)
	})
	assertGoldenOutput(t, "trace_summaries_raw.golden", func(buf *bytes.Buffer) error {
		return WriteTraceSummaries(buf, "raw", runs)
	})
}

func TestWriteThreadList_Golden(t *testing.T) {
	t.Parallel()

	threads := []corethreads.ThreadData{
		{
			ThreadID: "thread-1",
			Messages: []corethreads.Message{
				[]byte(`{"role":"user","content":"hello"}`),
				[]byte(`{"role":"assistant","content":"world"}`),
			},
		},
	}

	assertGoldenOutput(t, "thread_list_pretty.golden", func(buf *bytes.Buffer) error {
		return WriteThreadList(buf, "pretty", threads)
	})
	assertGoldenOutput(t, "thread_list_json.golden", func(buf *bytes.Buffer) error {
		return WriteThreadList(buf, "json", threads)
	})
	assertGoldenOutput(t, "thread_list_raw.golden", func(buf *bytes.Buffer) error {
		return WriteThreadList(buf, "raw", threads)
	})
}

func assertGoldenOutput(t *testing.T, fixture string, fn func(*bytes.Buffer) error) {
	t.Helper()

	var out bytes.Buffer
	if err := fn(&out); err != nil {
		t.Fatalf("render error = %v", err)
	}

	path := filepath.Join("testdata", fixture)
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", fixture, err)
	}
	if out.String() != string(want) {
		t.Fatalf("fixture %s mismatch\n--- got ---\n%s\n--- want ---\n%s", fixture, out.String(), string(want))
	}
}
