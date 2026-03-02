// thread_test.go validates single-thread service orchestration behavior.
package single

import (
	"context"
	"errors"
	"strings"
	"testing"

	langsmiththreads "langsmith-fetch-go/internal/langsmith/threads"
)

type fakeThreadsAccessor struct {
	params langsmiththreads.GetMessagesParams
	msgs   []langsmiththreads.Message
	err    error
	called bool
}

func (f *fakeThreadsAccessor) GetMessages(_ context.Context, params langsmiththreads.GetMessagesParams) ([]langsmiththreads.Message, error) {
	f.called = true
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return f.msgs, nil
}

func TestNewThreadService_RequiresAccessor(t *testing.T) {
	t.Parallel()

	svc, err := NewThreadService(nil)
	if err == nil {
		t.Fatal("NewThreadService(nil) error = nil, want non-nil")
	}
	if svc != nil {
		t.Fatal("NewThreadService(nil) service != nil, want nil")
	}
}

func TestGetMessages_RequiresThreadID(t *testing.T) {
	t.Parallel()

	accessor := &fakeThreadsAccessor{}
	svc, err := NewThreadService(accessor)
	if err != nil {
		t.Fatalf("NewThreadService() error = %v", err)
	}

	_, err = svc.GetMessages(context.Background(), ThreadParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "thread id is required") {
		t.Fatalf("GetMessages() error = %v, want thread id required", err)
	}
	if accessor.called {
		t.Fatal("GetMessages() called unexpectedly")
	}
}

func TestGetMessages_RequiresProjectID(t *testing.T) {
	t.Parallel()

	accessor := &fakeThreadsAccessor{}
	svc, err := NewThreadService(accessor)
	if err != nil {
		t.Fatalf("NewThreadService() error = %v", err)
	}

	_, err = svc.GetMessages(context.Background(), ThreadParams{ThreadID: "thread-123"})
	if err == nil || !strings.Contains(err.Error(), "project id is required") {
		t.Fatalf("GetMessages() error = %v, want project id required", err)
	}
	if accessor.called {
		t.Fatal("GetMessages() called unexpectedly")
	}
}

func TestGetMessages_PassesParamsAndReturnsMessages(t *testing.T) {
	t.Parallel()

	accessor := &fakeThreadsAccessor{
		msgs: []langsmiththreads.Message{
			[]byte(`{"role":"user","content":"hello"}`),
		},
	}
	svc, err := NewThreadService(accessor)
	if err != nil {
		t.Fatalf("NewThreadService() error = %v", err)
	}

	msgs, err := svc.GetMessages(context.Background(), ThreadParams{
		ThreadID:  "thread-123",
		ProjectID: "project-123",
	})
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}
	if accessor.params.ThreadID != "thread-123" || accessor.params.ProjectID != "project-123" {
		t.Fatalf("params = %+v, want thread-123/project-123", accessor.params)
	}
	if len(msgs) != 1 || !strings.Contains(string(msgs[0]), `"hello"`) {
		t.Fatalf("msgs = %q, want returned message", string(msgs[0]))
	}
}

func TestGetThreadMessages_PropagatesAccessorError(t *testing.T) {
	t.Parallel()

	accessor := &fakeThreadsAccessor{err: errors.New("network failed")}
	svc, err := NewThreadService(accessor)
	if err != nil {
		t.Fatalf("NewThreadService() error = %v", err)
	}

	_, err = svc.GetMessages(context.Background(), ThreadParams{
		ThreadID:  "thread-123",
		ProjectID: "project-123",
	})
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("GetMessages() error = %v, want wrapped accessor error", err)
	}
}
