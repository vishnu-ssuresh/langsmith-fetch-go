// integration_test.go adds end-to-end command tests against a mock HTTP API.
package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"langsmith-fetch-go/internal/config"
)

type capturedRequest struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   []byte
}

func newMockLangSmithServer(
	t *testing.T,
	handler func(http.ResponseWriter, capturedRequest),
) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusInternalServerError)
			return
		}
		if err := r.Body.Close(); err != nil {
			http.Error(w, "failed to close request body", http.StatusInternalServerError)
			return
		}

		handler(w, capturedRequest{
			Method: r.Method,
			Path:   r.URL.Path,
			Query:  r.URL.RawQuery,
			Header: r.Header.Clone(),
			Body:   body,
		})
	}))
}

func TestExecute_Trace_Integration(t *testing.T) {
	t.Parallel()

	requestCh := make(chan capturedRequest, 1)
	server := newMockLangSmithServer(t, func(w http.ResponseWriter, req capturedRequest) {
		requestCh <- req
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(
			w,
			`{"id":"trace-123","messages":[{"role":"user","content":"hello"}]}`,
		)
	})
	defer server.Close()

	deps := NewDeps()
	deps.LoadConfig = func() config.Values {
		return config.Values{
			APIKey:   "integration-api-key",
			Endpoint: server.URL,
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Execute(
		[]string{"trace", "--trace-id", "trace-123", "--format", "json"},
		&stdout,
		&stderr,
		deps,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var req capturedRequest
	select {
	case req = <-requestCh:
	default:
		t.Fatal("mock server did not receive request")
	}

	if req.Method != http.MethodGet {
		t.Fatalf("request method = %q, want %q", req.Method, http.MethodGet)
	}
	if req.Path != "/runs/trace-123" {
		t.Fatalf("request path = %q, want %q", req.Path, "/runs/trace-123")
	}
	if !strings.Contains(req.Query, "include_messages=true") {
		t.Fatalf("request query = %q, want include_messages=true", req.Query)
	}
	if got := req.Header.Get("X-API-Key"); got != "integration-api-key" {
		t.Fatalf("X-API-Key = %q, want %q", got, "integration-api-key")
	}

	if got := stdout.String(); !strings.Contains(got, `"role": "user"`) {
		t.Fatalf("stdout = %q, want JSON trace message", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestExecute_Traces_Integration(t *testing.T) {
	t.Parallel()

	requestCh := make(chan capturedRequest, 1)
	server := newMockLangSmithServer(t, func(w http.ResponseWriter, req capturedRequest) {
		requestCh <- req
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(
			w,
			`{"runs":[{"id":"trace-1","name":"Run One","start_time":"2026-01-01T00:00:00Z"},{"id":"trace-2","name":"Run Two","start_time":"2026-01-01T01:00:00Z"}]}`,
		)
	})
	defer server.Close()

	deps := NewDeps()
	deps.LoadConfig = func() config.Values {
		return config.Values{
			APIKey:   "integration-api-key",
			Endpoint: server.URL,
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Execute(
		[]string{
			"traces",
			"--project-uuid", "project-123",
			"--limit", "2",
			"--format", "json",
			"--no-progress",
		},
		&stdout,
		&stderr,
		deps,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var req capturedRequest
	select {
	case req = <-requestCh:
	default:
		t.Fatal("mock server did not receive request")
	}

	if req.Method != http.MethodPost {
		t.Fatalf("request method = %q, want %q", req.Method, http.MethodPost)
	}
	if req.Path != "/runs/query" {
		t.Fatalf("request path = %q, want %q", req.Path, "/runs/query")
	}
	if got := req.Header.Get("X-API-Key"); got != "integration-api-key" {
		t.Fatalf("X-API-Key = %q, want %q", got, "integration-api-key")
	}

	var body struct {
		Session   []string `json:"session"`
		IsRoot    bool     `json:"is_root"`
		Limit     int      `json:"limit"`
		StartTime string   `json:"start_time"`
	}
	if err := json.Unmarshal(req.Body, &body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	if len(body.Session) != 1 || body.Session[0] != "project-123" {
		t.Fatalf("session = %#v, want [project-123]", body.Session)
	}
	if !body.IsRoot {
		t.Fatalf("is_root = %v, want true", body.IsRoot)
	}
	if body.Limit != 2 {
		t.Fatalf("limit = %d, want 2", body.Limit)
	}
	if body.StartTime != "" {
		t.Fatalf("start_time = %q, want empty", body.StartTime)
	}

	if got := stdout.String(); !strings.Contains(got, `"id": "trace-1"`) {
		t.Fatalf("stdout = %q, want first trace JSON output", got)
	}
	if got := stdout.String(); !strings.Contains(got, `"id": "trace-2"`) {
		t.Fatalf("stdout = %q, want second trace JSON output", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestExecute_Thread_Integration(t *testing.T) {
	t.Parallel()

	requestCh := make(chan capturedRequest, 1)
	server := newMockLangSmithServer(t, func(w http.ResponseWriter, req capturedRequest) {
		requestCh <- req
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(
			w,
			`{"previews":{"all_messages":"{\"role\":\"user\",\"content\":\"hello\"}\n\n{\"role\":\"assistant\",\"content\":\"world\"}"}}`,
		)
	})
	defer server.Close()

	deps := NewDeps()
	deps.LoadConfig = func() config.Values {
		return config.Values{
			APIKey:   "integration-api-key",
			Endpoint: server.URL,
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Execute(
		[]string{
			"thread",
			"--project-uuid", "project-123",
			"--thread-id", "thread-abc",
			"--format", "json",
		},
		&stdout,
		&stderr,
		deps,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var req capturedRequest
	select {
	case req = <-requestCh:
	default:
		t.Fatal("mock server did not receive request")
	}

	if req.Method != http.MethodGet {
		t.Fatalf("request method = %q, want %q", req.Method, http.MethodGet)
	}
	if req.Path != "/runs/threads/thread-abc" {
		t.Fatalf("request path = %q, want %q", req.Path, "/runs/threads/thread-abc")
	}
	if !strings.Contains(req.Query, "select=all_messages") {
		t.Fatalf("request query = %q, want select=all_messages", req.Query)
	}
	if !strings.Contains(req.Query, "session_id=project-123") {
		t.Fatalf("request query = %q, want session_id=project-123", req.Query)
	}
	if got := req.Header.Get("X-API-Key"); got != "integration-api-key" {
		t.Fatalf("X-API-Key = %q, want %q", got, "integration-api-key")
	}

	if got := stdout.String(); !strings.Contains(got, `"role": "user"`) {
		t.Fatalf("stdout = %q, want user message JSON output", got)
	}
	if got := stdout.String(); !strings.Contains(got, `"role": "assistant"`) {
		t.Fatalf("stdout = %q, want assistant message JSON output", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
