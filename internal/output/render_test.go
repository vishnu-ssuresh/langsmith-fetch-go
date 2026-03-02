// render_test.go validates output rendering across raw/json/pretty formats.
package output

import (
	"bytes"
	"strings"
	"testing"

	coresingle "langsmith-fetch-go/internal/core/single"
	corethreads "langsmith-fetch-go/internal/core/threads"
	coretraces "langsmith-fetch-go/internal/core/traces"
)

func TestWriteTraceMessages_Pretty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := WriteTraceMessages(&out, "pretty", []coresingle.Message{
		[]byte(`{"role":"user","content":"hello"}`),
	})
	if err != nil {
		t.Fatalf("WriteTraceMessages() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "[1] {\"role\":\"user\"") {
		t.Fatalf("stdout = %q, want pretty message output", got)
	}
}

func TestWriteTraceSummaries_Pretty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := WriteTraceSummaries(&out, "pretty", []coretraces.Summary{
		{ID: "trace-1", Name: "hello", StartTime: "2026-01-01T00:00:00Z"},
	})
	if err != nil {
		t.Fatalf("WriteTraceSummaries() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "trace-1\thello") {
		t.Fatalf("stdout = %q, want pretty summaries", got)
	}
}

func TestWriteThreadList_Pretty(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := WriteThreadList(&out, "pretty", []corethreads.ThreadData{
		{
			ThreadID: "thread-1",
			Messages: []corethreads.Message{
				[]byte(`{"role":"assistant","content":"hi"}`),
			},
		},
	})
	if err != nil {
		t.Fatalf("WriteThreadList() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Thread: thread-1") || !strings.Contains(got, "[1] {\"role\":\"assistant\"") {
		t.Fatalf("stdout = %q, want pretty thread list", got)
	}
}

func TestWriteThreadMessages_JSON(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := WriteThreadMessages(&out, "json", []coresingle.ThreadMessage{
		[]byte(`{"role":"assistant","content":"hi"}`),
	})
	if err != nil {
		t.Fatalf("WriteThreadMessages() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "\"role\": \"assistant\"") {
		t.Fatalf("stdout = %q, want JSON output", got)
	}
}

func TestWriteTraceMessages_UnsupportedFormat(t *testing.T) {
	t.Parallel()

	err := WriteTraceMessages(&bytes.Buffer{}, "xml", nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported format") {
		t.Fatalf("WriteTraceMessages() error = %v, want unsupported format error", err)
	}
}
