package projects

import (
	"context"
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

func TestResolveProjectUUID_RequiresProjectName(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected request")
	})
	accessor, _ := NewAccessor(client)

	_, err := accessor.ResolveProjectUUID(context.Background(), "   ")
	if err == nil || !strings.Contains(err.Error(), "project name is required") {
		t.Fatalf("ResolveProjectUUID() error = %v, want project name required", err)
	}
}

func TestResolveProjectUUID_ParsesResponse(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[{"id":"proj-123","name":"my-project","tenant_id":"t","start_time":"2026-01-01T00:00:00Z"}]`)
	})
	accessor, _ := NewAccessor(client)

	projectID, err := accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err != nil {
		t.Fatalf("ResolveProjectUUID() error = %v", err)
	}
	if projectID != "proj-123" {
		t.Fatalf("projectID = %q, want %q", projectID, "proj-123")
	}
}

func TestResolveProjectUUID_NotFound(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[]`)
	})
	accessor, _ := NewAccessor(client)

	_, err := accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("ResolveProjectUUID() error = %v, want not found error", err)
	}
}

func TestResolveProjectUUID_PropagatesError(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"detail":"forbidden"}`)
	})
	accessor, _ := NewAccessor(client)

	_, err := accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err == nil {
		t.Fatal("ResolveProjectUUID() error = nil, want non-nil")
	}
}
