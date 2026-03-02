// trace_test.go validates single-trace service orchestration behavior.
package single

import (
	"context"
	"errors"
	"strings"
	"testing"

	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
)

type fakeRunsAccessor struct {
	params langsmithruns.GetRunParams
	run    langsmithruns.Run
	err    error
	called bool
}

func (f *fakeRunsAccessor) GetRun(_ context.Context, params langsmithruns.GetRunParams) (langsmithruns.Run, error) {
	f.called = true
	f.params = params
	if f.err != nil {
		return langsmithruns.Run{}, f.err
	}
	return f.run, nil
}

func TestNewTraceService_RequiresAccessor(t *testing.T) {
	t.Parallel()

	svc, err := NewTraceService(nil)
	if err == nil {
		t.Fatal("NewTraceService(nil) error = nil, want non-nil")
	}
	if svc != nil {
		t.Fatal("NewTraceService(nil) service != nil, want nil")
	}
}

func TestGetMessages_RequiresTraceID(t *testing.T) {
	t.Parallel()

	accessor := &fakeRunsAccessor{}
	svc, err := NewTraceService(accessor)
	if err != nil {
		t.Fatalf("NewTraceService() error = %v", err)
	}

	_, err = svc.GetMessages(context.Background(), TraceParams{})
	if err == nil || !strings.Contains(err.Error(), "trace id is required") {
		t.Fatalf("GetMessages() error = %v, want trace id required", err)
	}
	if accessor.called {
		t.Fatal("GetRun() called unexpectedly")
	}
}

func TestGetMessages_PrefersRunMessages(t *testing.T) {
	t.Parallel()

	accessor := &fakeRunsAccessor{
		run: langsmithruns.Run{
			Messages: []langsmithruns.Message{
				[]byte(`{"role":"user","content":"hello"}`),
			},
			Outputs: langsmithruns.Outputs{
				Messages: []langsmithruns.Message{
					[]byte(`{"role":"assistant","content":"fallback"}`),
				},
			},
		},
	}
	svc, err := NewTraceService(accessor)
	if err != nil {
		t.Fatalf("NewTraceService() error = %v", err)
	}

	msgs, err := svc.GetMessages(context.Background(), TraceParams{TraceID: "trace-1"})
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}
	if accessor.params.RunID != "trace-1" || !accessor.params.IncludeMessages {
		t.Fatalf("params = %+v, want runID=trace-1 includeMessages=true", accessor.params)
	}
	if len(msgs) != 1 || !strings.Contains(string(msgs[0]), `"hello"`) {
		t.Fatalf("msgs = %q, want run.messages entry", string(msgs[0]))
	}
}

func TestGetMessages_FallsBackToOutputMessages(t *testing.T) {
	t.Parallel()

	accessor := &fakeRunsAccessor{
		run: langsmithruns.Run{
			Messages: nil,
			Outputs: langsmithruns.Outputs{
				Messages: []langsmithruns.Message{
					[]byte(`{"role":"assistant","content":"from-output"}`),
				},
			},
		},
	}
	svc, err := NewTraceService(accessor)
	if err != nil {
		t.Fatalf("NewTraceService() error = %v", err)
	}

	msgs, err := svc.GetMessages(context.Background(), TraceParams{TraceID: "trace-1"})
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}
	if len(msgs) != 1 || !strings.Contains(string(msgs[0]), `"from-output"`) {
		t.Fatalf("msgs = %q, want outputs.messages entry", string(msgs[0]))
	}
}

func TestGetMessages_PropagatesAccessorError(t *testing.T) {
	t.Parallel()

	accessor := &fakeRunsAccessor{err: errors.New("network failed")}
	svc, err := NewTraceService(accessor)
	if err != nil {
		t.Fatalf("NewTraceService() error = %v", err)
	}

	_, err = svc.GetMessages(context.Background(), TraceParams{TraceID: "trace-1"})
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("GetMessages() error = %v, want wrapped accessor error", err)
	}
}
