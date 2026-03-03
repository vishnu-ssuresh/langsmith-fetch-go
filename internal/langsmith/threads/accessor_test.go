package threads

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/option"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *langsmith.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return langsmith.NewClient(
		option.WithBaseURL(server.URL),
		option.WithAPIKey("test-key"),
	)
}

func TestNewAccessor_RequiresClient(t *testing.T) {
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
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected request")
	})
	accessor, _ := NewAccessor(client)

	_, err := accessor.GetMessages(context.Background(), GetMessagesParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "thread id is required") {
		t.Fatalf("GetMessages() error = %v, want thread id required", err)
	}
}

func TestGetMessages_RequiresProjectID(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected request")
	})
	accessor, _ := NewAccessor(client)

	_, err := accessor.GetMessages(context.Background(), GetMessagesParams{ThreadID: "thread-123"})
	if err == nil || !strings.Contains(err.Error(), "project id is required") {
		t.Fatalf("GetMessages() error = %v, want project id required", err)
	}
}

func TestGetMessages_ParsesMessages(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/runs/threads/thread-123" {
			t.Fatalf("r.URL.Path = %q, want %q", r.URL.Path, "/api/v1/runs/threads/thread-123")
		}
		if got := r.URL.Query().Get("select"); got != "all_messages" {
			t.Fatalf("select query = %q, want %q", got, "all_messages")
		}
		if got := r.URL.Query().Get("session_id"); got != "project-123" {
			t.Fatalf("session_id query = %q, want %q", got, "project-123")
		}
		body, _ := io.ReadAll(r.Body)
		if strings.TrimSpace(string(body)) != "" {
			t.Fatalf("GET body should be empty, got %q", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"previews": {
				"all_messages": "{\"role\":\"user\",\"content\":\"hello\"}\n\n{\"role\":\"assistant\",\"content\":\"hi\"}"
			}
		}`)
	})
	accessor, _ := NewAccessor(client)

	messages, err := accessor.GetMessages(context.Background(), GetMessagesParams{
		ThreadID:  "thread-123",
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
}

func TestGetMessages_MissingAllMessages(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"previews":{}}`)
	})
	accessor, _ := NewAccessor(client)

	_, err := accessor.GetMessages(context.Background(), GetMessagesParams{
		ThreadID:  "thread-123",
		ProjectID: "project-123",
	})
	if err == nil || !strings.Contains(err.Error(), "missing previews.all_messages") {
		t.Fatalf("GetMessages() error = %v, want missing all_messages error", err)
	}
}

func TestGetMessages_PropagatesError(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"detail":"not found"}`)
	})
	accessor, _ := NewAccessor(client)

	_, err := accessor.GetMessages(context.Background(), GetMessagesParams{
		ThreadID:  "thread-123",
		ProjectID: "project-123",
	})
	if err == nil {
		t.Fatal("GetMessages() error = nil, want non-nil")
	}
}
