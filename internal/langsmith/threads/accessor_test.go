// accessor_test.go validates thread accessor request and parsing behavior.
package threads

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"langsmith-sdk/go/langsmith/transport"
)

type fakeDoer struct {
	req    transport.Request
	resp   transport.Response
	err    error
	called bool
}

func (f *fakeDoer) Do(_ context.Context, req transport.Request) (transport.Response, error) {
	f.called = true
	f.req = req
	return f.resp, f.err
}

func TestNewAccessor_RequiresDoer(t *testing.T) {
	t.Parallel()

	accessor, err := NewAccessor(nil)
	if err == nil {
		t.Fatal("NewAccessor(nil) error = nil, want non-nil")
	}
	if accessor != nil {
		t.Fatal("NewAccessor(nil) accessor != nil, want nil")
	}
}

func TestGetMessages_RequiresThreadID(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetMessages(context.Background(), GetMessagesParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "thread id is required") {
		t.Fatalf("GetMessages() error = %v, want thread id required", err)
	}
	if doer.called {
		t.Fatal("Do() called unexpectedly")
	}
}

func TestGetMessages_RequiresProjectID(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetMessages(context.Background(), GetMessagesParams{ThreadID: "thread-123"})
	if err == nil || !strings.Contains(err.Error(), "project id is required") {
		t.Fatalf("GetMessages() error = %v, want project id required", err)
	}
	if doer.called {
		t.Fatal("Do() called unexpectedly")
	}
}

func TestGetMessages_BuildsRequestAndParsesMessages(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body: []byte(`{
  "previews": {
    "all_messages": "{\"role\":\"user\",\"content\":\"hello\"}\n\n{\"role\":\"assistant\",\"content\":\"hi\"}"
  }
}`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	messages, err := accessor.GetMessages(context.Background(), GetMessagesParams{
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

	if doer.req.Method != http.MethodGet {
		t.Fatalf("Method = %q, want GET", doer.req.Method)
	}
	if doer.req.Path != "/runs/threads/thread%2Fa%20b" {
		t.Fatalf("Path = %q, want escaped path", doer.req.Path)
	}

	wantQuery := url.Values{
		"select":     []string{"all_messages"},
		"session_id": []string{"project-123"},
	}
	if got := doer.req.Query.Encode(); got != wantQuery.Encode() {
		t.Fatalf("Query = %q, want %q", got, wantQuery.Encode())
	}
}

func TestGetMessages_PropagatesDoError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{err: errors.New("network failed")}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetMessages(context.Background(), GetMessagesParams{
		ThreadID:  "thread-123",
		ProjectID: "project-123",
	})
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("GetMessages() error = %v, want wrapped do error", err)
	}
}

func TestGetMessages_StatusError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusBadRequest,
			Body:       []byte("bad request"),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetMessages(context.Background(), GetMessagesParams{
		ThreadID:  "thread-123",
		ProjectID: "project-123",
	})
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("GetMessages() error = %v, want status error", err)
	}
}

func TestGetMessages_DecodeResponseError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"previews"`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetMessages(context.Background(), GetMessagesParams{
		ThreadID:  "thread-123",
		ProjectID: "project-123",
	})
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("GetMessages() error = %v, want decode response error", err)
	}
}

func TestGetMessages_MissingAllMessagesError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"previews":{}}`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetMessages(context.Background(), GetMessagesParams{
		ThreadID:  "thread-123",
		ProjectID: "project-123",
	})
	if err == nil || !strings.Contains(err.Error(), "missing previews.all_messages") {
		t.Fatalf("GetMessages() error = %v, want missing all_messages error", err)
	}
}

func TestGetMessages_DecodeMessageError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body: []byte(`{
  "previews": {
    "all_messages": "{\"role\":\"user\",\"content\":\"hello\"}\n\n{\"role\":"
  }
}`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetMessages(context.Background(), GetMessagesParams{
		ThreadID:  "thread-123",
		ProjectID: "project-123",
	})
	if err == nil || !strings.Contains(err.Error(), "decode message") {
		t.Fatalf("GetMessages() error = %v, want decode message error", err)
	}
}
