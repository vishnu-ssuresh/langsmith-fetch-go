package feedback

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

func TestListByRuns_RequiresRunIDs(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected request")
	})
	accessor, _ := NewAccessor(client)

	_, err := accessor.ListByRuns(context.Background(), ListParams{})
	if err == nil || !strings.Contains(err.Error(), "at least one run id is required") {
		t.Fatalf("ListByRuns() error = %v, want run id required", err)
	}
}

func TestListByRuns_ParsesResponse(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `[
			{"id":"fb-1","run_id":"run-1","key":"correctness","score":1,"value":"good","comment":"ok"}
		]`)
	})
	accessor, _ := NewAccessor(client)

	items, err := accessor.ListByRuns(context.Background(), ListParams{
		RunIDs: []string{"run-1"},
	})
	if err != nil {
		t.Fatalf("ListByRuns() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != "fb-1" {
		t.Fatalf("items = %+v, want one item fb-1", items)
	}
}

func TestListByRuns_PropagatesError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"detail":"rate limited"}`)
	})
	accessor, _ := NewAccessor(client)

	_, err := accessor.ListByRuns(context.Background(), ListParams{
		RunIDs: []string{"run-1"},
	})
	if err == nil {
		t.Fatal("ListByRuns() error = nil, want non-nil")
	}
}
