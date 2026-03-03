package runs

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

func TestQueryRoot_RequiresProjectID(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected request")
	})
	accessor, err := NewAccessor(client)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}
	_, err = accessor.QueryRoot(context.Background(), QueryRootParams{})
	if err == nil || !strings.Contains(err.Error(), "project id is required") {
		t.Fatalf("QueryRoot() error = %v, want project id required", err)
	}
}

func TestQueryRoot_DefaultLimitAndDecode(t *testing.T) {
	t.Parallel()
	var capturedBody []byte
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"runs":[{"id":"run-1","name":"trace-a","start_time":"2026-01-01T00:00:00Z"}],"cursors":{}}`)
	})
	accessor, _ := NewAccessor(client)

	runs, err := accessor.QueryRoot(context.Background(), QueryRootParams{ProjectID: "project-123"})
	if err != nil {
		t.Fatalf("QueryRoot() error = %v", err)
	}
	if len(runs) != 1 || runs[0].ID != "run-1" {
		t.Fatalf("runs = %+v, want 1 run with ID run-1", runs)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if limit, ok := body["limit"].(float64); !ok || int(limit) != 20 {
		t.Fatalf("limit = %v, want 20", body["limit"])
	}
	if isRoot, ok := body["is_root"].(bool); !ok || !isRoot {
		t.Fatalf("is_root = %v, want true", body["is_root"])
	}
}

func TestQueryRootRuns_ParsesThreadIDs(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"runs":[
			{"id":"run-1","name":"trace-a","start_time":"2026-01-01T00:00:00Z","trace_id":"run-1","run_type":"chain","session_id":"s","status":"ok","dotted_order":"d","app_path":"a","thread_id":"thread-1","extra":{"metadata":{"thread_id":"thread-1"}}},
			{"id":"run-2","name":"trace-b","start_time":"2026-01-01T01:00:00Z","trace_id":"run-2","run_type":"chain","session_id":"s","status":"ok","dotted_order":"d","app_path":"a","thread_id":"thread-2","extra":{"metadata":{"thread_id":"thread-2"}}}
		],"cursors":{}}`)
	})
	accessor, _ := NewAccessor(client)

	runs, err := accessor.QueryRootRuns(context.Background(), QueryRootParams{ProjectID: "project-123"})
	if err != nil {
		t.Fatalf("QueryRootRuns() error = %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}
	if runs[0].ThreadID != "thread-1" {
		t.Fatalf("runs[0].ThreadID = %q, want %q", runs[0].ThreadID, "thread-1")
	}
	if runs[1].ThreadID != "thread-2" {
		t.Fatalf("runs[1].ThreadID = %q, want %q", runs[1].ThreadID, "thread-2")
	}
}

func TestGetRun_RequiresRunID(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("unexpected request")
	})
	accessor, _ := NewAccessor(client)
	_, err := accessor.GetRun(context.Background(), GetRunParams{})
	if err == nil || !strings.Contains(err.Error(), "run id is required") {
		t.Fatalf("GetRun() error = %v, want run id required", err)
	}
}

func TestGetRun_DecodesResponse(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/runs/trace-1" {
			t.Fatalf("r.URL.Path = %q, want %q", r.URL.Path, "/api/v1/runs/trace-1")
		}
		if got := r.URL.Query().Get("include_messages"); got != "true" {
			t.Fatalf("include_messages query = %q, want %q", got, "true")
		}
		body, _ := io.ReadAll(r.Body)
		if strings.TrimSpace(string(body)) != "" {
			t.Fatalf("GET body should be empty, got %q", string(body))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"id":"trace-1","status":"completed",
			"start_time":"2026-01-01T00:00:00Z","end_time":"2026-01-01T00:00:02Z",
			"prompt_tokens":11,"total_tokens":22,"total_cost":0.42,
			"first_token_time":"2026-01-01T00:00:00.100Z",
			"feedback_stats":{"correctness":1},
			"extra":{"metadata":{"thread_id":"thread-1"}},
			"messages":[{"role":"user","content":"hi"}],
			"outputs":{"messages":[{"role":"assistant","content":"hello"}]}
		}`)
	})
	accessor, _ := NewAccessor(client)

	run, err := accessor.GetRun(context.Background(), GetRunParams{RunID: "trace-1", IncludeMessages: true})
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if run.ID != "trace-1" {
		t.Fatalf("run.ID = %q, want %q", run.ID, "trace-1")
	}
	if run.Status != "completed" {
		t.Fatalf("run.Status = %q, want %q", run.Status, "completed")
	}
	if len(run.Messages) != 1 {
		t.Fatalf("len(run.Messages) = %d, want 1", len(run.Messages))
	}
	if run.PromptTokens == nil || *run.PromptTokens != 11 {
		t.Fatalf("run.PromptTokens = %+v, want 11", run.PromptTokens)
	}
	if run.TotalCost == nil || *run.TotalCost != 0.42 {
		t.Fatalf("run.TotalCost = %+v, want 0.42", run.TotalCost)
	}
}

func TestGetRun_PropagatesError(t *testing.T) {
	t.Parallel()
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"detail":"not found"}`)
	})
	accessor, _ := NewAccessor(client)

	_, err := accessor.GetRun(context.Background(), GetRunParams{RunID: "trace-1"})
	if err == nil {
		t.Fatal("GetRun() error = nil, want non-nil")
	}
}
