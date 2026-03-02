// service_test.go validates thread service orchestration behavior.
package threads

import (
	"context"
	"encoding/json"
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

func TestNew_RequiresThreadsAccessor(t *testing.T) {
	t.Parallel()

	svc, err := New(nil)
	if err == nil {
		t.Fatal("New(nil) error = nil, want non-nil")
	}
	if svc != nil {
		t.Fatal("New(nil) service != nil, want nil")
	}
}

func TestGetMessages_RequiresThreadID(t *testing.T) {
	t.Parallel()

	accessor := &fakeThreadsAccessor{}
	svc, err := New(accessor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.GetMessages(context.Background(), GetParams{ProjectID: "project-123"})
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
	svc, err := New(accessor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.GetMessages(context.Background(), GetParams{ThreadID: "thread-123"})
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
			[]byte(`{"role":"assistant","content":"hi"}`),
		},
	}
	svc, err := New(accessor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	messages, err := svc.GetMessages(context.Background(), GetParams{
		ThreadID:  "thread/a b",
		ProjectID: "project-123",
	})
	if err != nil {
		t.Fatalf("GetMessages() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
	}
	type messageView struct {
		Role string `json:"role"`
	}
	var first messageView
	if err := json.Unmarshal(messages[0], &first); err != nil {
		t.Fatalf("json.Unmarshal(messages[0]) error = %v", err)
	}
	if first.Role != "user" {
		t.Fatalf("messages[0].Role = %q, want %q", first.Role, "user")
	}
	var second messageView
	if err := json.Unmarshal(messages[1], &second); err != nil {
		t.Fatalf("json.Unmarshal(messages[1]) error = %v", err)
	}
	if second.Role != "assistant" {
		t.Fatalf("messages[1].Role = %q, want %q", second.Role, "assistant")
	}
	if accessor.params.ThreadID != "thread/a b" {
		t.Fatalf("ThreadID = %q, want %q", accessor.params.ThreadID, "thread/a b")
	}
	if accessor.params.ProjectID != "project-123" {
		t.Fatalf("ProjectID = %q, want %q", accessor.params.ProjectID, "project-123")
	}
}

func TestGetMessages_PropagatesAccessorError(t *testing.T) {
	t.Parallel()

	accessor := &fakeThreadsAccessor{err: errors.New("network failed")}
	svc, err := New(accessor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.GetMessages(context.Background(), GetParams{
		ThreadID:  "thread-123",
		ProjectID: "project-123",
	})
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("GetMessages() error = %v, want wrapped do error", err)
	}
}
