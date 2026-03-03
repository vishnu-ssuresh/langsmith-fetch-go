// integration_test.go adds end-to-end command tests against a mock HTTP API.
package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/langchain-ai/langsmith-go"

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
	if !strings.Contains(req.Path, "trace-123") {
		t.Fatalf("request path = %q, want path containing trace-123", req.Path)
	}
	if got := req.Header.Get("X-Api-Key"); got != "integration-api-key" {
		t.Fatalf("X-Api-Key = %q, want %q", got, "integration-api-key")
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
			`{"runs":[{"id":"trace-1","name":"Run One","start_time":"2026-01-01T00:00:00Z","trace_id":"trace-1","run_type":"chain","session_id":"project-123","status":"ok","dotted_order":"d","app_path":"a"},{"id":"trace-2","name":"Run Two","start_time":"2026-01-01T01:00:00Z","trace_id":"trace-2","run_type":"chain","session_id":"project-123","status":"ok","dotted_order":"d","app_path":"a"}],"cursors":{}}`,
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
	if got := req.Header.Get("X-Api-Key"); got != "integration-api-key" {
		t.Fatalf("X-Api-Key = %q, want %q", got, "integration-api-key")
	}

	var body struct {
		Session []string `json:"session"`
		IsRoot  bool     `json:"is_root"`
		Limit   int      `json:"limit"`
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

	if got := stdout.String(); !strings.Contains(got, `"id": "trace-1"`) {
		t.Fatalf("stdout = %q, want first trace JSON output", got)
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
	if !strings.Contains(req.Path, "thread-abc") {
		t.Fatalf("request path = %q, want path containing thread-abc", req.Path)
	}
	if got := req.Header.Get("X-Api-Key"); got != "integration-api-key" {
		t.Fatalf("X-Api-Key = %q, want %q", got, "integration-api-key")
	}

	if got := stdout.String(); !strings.Contains(got, `"role": "user"`) {
		t.Fatalf("stdout = %q, want user message JSON output", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestExecute_Threads_Integration(t *testing.T) {
	t.Parallel()

	requestCh := make(chan capturedRequest, 8)
	server := newMockLangSmithServer(t, func(w http.ResponseWriter, req capturedRequest) {
		requestCh <- req
		w.Header().Set("Content-Type", "application/json")

		if strings.Contains(req.Path, "runs/query") || (req.Method == http.MethodPost && strings.HasSuffix(req.Path, "runs/query")) {
			_, _ = io.WriteString(
				w,
				`{"runs":[{"id":"run-1","name":"one","start_time":"2026-01-01T00:00:00Z","trace_id":"run-1","run_type":"chain","session_id":"project-123","status":"ok","dotted_order":"d","app_path":"a","thread_id":"thread-a","extra":{"metadata":{"thread_id":"thread-a"}}},{"id":"run-2","name":"two","start_time":"2026-01-01T00:01:00Z","trace_id":"run-2","run_type":"chain","session_id":"project-123","status":"ok","dotted_order":"d","app_path":"a","thread_id":"thread-b","extra":{"metadata":{"thread_id":"thread-b"}}},{"id":"run-3","name":"three","start_time":"2026-01-01T00:02:00Z","trace_id":"run-3","run_type":"chain","session_id":"project-123","status":"ok","dotted_order":"d","app_path":"a","thread_id":"thread-a","extra":{"metadata":{"thread_id":"thread-a"}}}],"cursors":{}}`,
			)
		} else if strings.Contains(req.Path, "thread-a") {
			_, _ = io.WriteString(
				w,
				`{"previews":{"all_messages":"{\"role\":\"user\",\"content\":\"hello from a\"}"}}`,
			)
		} else if strings.Contains(req.Path, "thread-b") {
			_, _ = io.WriteString(
				w,
				`{"previews":{"all_messages":"{\"role\":\"assistant\",\"content\":\"hello from b\"}"}}`,
			)
		} else {
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
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
			"threads",
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

	if got := stdout.String(); !strings.Contains(got, `"thread_id": "thread-a"`) {
		t.Fatalf("stdout = %q, want thread-a output", got)
	}
	if got := stdout.String(); !strings.Contains(got, `"thread_id": "thread-b"`) {
		t.Fatalf("stdout = %q, want thread-b output", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestExecute_ConfigShow_Integration(t *testing.T) {
	t.Parallel()

	deps := NewDeps()
	deps.LoadConfig = func() config.Values {
		return config.Values{
			APIKey:        "lsv2_pt_1234567890",
			WorkspaceID:   "workspace-123",
			Endpoint:      "https://api.smith.langchain.com",
			ProjectUUID:   "project-uuid-123",
			ProjectName:   "demo-project",
			DefaultFormat: "json",
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Execute([]string{"config", "show"}, &stdout, &stderr, deps)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Current configuration:") {
		t.Fatalf("stdout = %q, want config header", output)
	}
	if !strings.Contains(output, "api_key: lsv2_pt_...") {
		t.Fatalf("stdout = %q, want masked api key", output)
	}
	if !strings.Contains(output, "workspace_id: workspace-123") {
		t.Fatalf("stdout = %q, want workspace_id", output)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestExecute_Trace_Integration_RetriesOnRateLimit(t *testing.T) {
	t.Parallel()

	var attempt atomic.Int64
	server := newMockLangSmithServer(t, func(w http.ResponseWriter, req capturedRequest) {
		current := attempt.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if current <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":"rate limited"}`)
			return
		}
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
	err := Execute(
		[]string{"trace", "--trace-id", "trace-123", "--format", "json"},
		&stdout,
		&bytes.Buffer{},
		deps,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := attempt.Load(); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
	if got := stdout.String(); !strings.Contains(got, `"role": "user"`) {
		t.Fatalf("stdout = %q, want JSON trace output", got)
	}
}

func TestExecute_Trace_Integration_SurfacesUnauthorizedError(t *testing.T) {
	t.Parallel()

	var attempt atomic.Int64
	server := newMockLangSmithServer(t, func(w http.ResponseWriter, req capturedRequest) {
		attempt.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"unauthorized"}`)
	})
	defer server.Close()

	deps := NewDeps()
	deps.LoadConfig = func() config.Values {
		return config.Values{
			APIKey:   "integration-api-key",
			Endpoint: server.URL,
		}
	}

	err := Execute(
		[]string{"trace", "--trace-id", "trace-123", "--format", "json"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		deps,
	)
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	var apiErr *langsmith.Error
	if !errors.As(err, &apiErr) {
		t.Fatalf("Execute() error = %v, want *langsmith.Error", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
}

func TestExecute_Trace_Integration_UsesSelfHostEndpointFromEnv(t *testing.T) {
	requestCh := make(chan capturedRequest, 1)
	server := newMockLangSmithServer(t, func(w http.ResponseWriter, req capturedRequest) {
		requestCh <- req
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(
			w,
			`{"id":"trace-self-host","messages":[{"role":"user","content":"from self host"}]}`,
		)
	})
	defer server.Close()

	t.Setenv("LANGSMITH_API_KEY", "integration-api-key")
	t.Setenv("LANGSMITH_ENDPOINT", server.URL)
	t.Setenv("LANGCHAIN_API_KEY", "")
	t.Setenv("LANGCHAIN_ENDPOINT", "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Execute(
		[]string{"trace", "--trace-id", "trace-self-host", "--format", "json"},
		&stdout,
		&stderr,
		NewDeps(),
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	var req capturedRequest
	select {
	case req = <-requestCh:
	default:
		t.Fatal("mock self-host server did not receive request")
	}

	if req.Method != http.MethodGet {
		t.Fatalf("request method = %q, want %q", req.Method, http.MethodGet)
	}
	if !strings.Contains(req.Path, "trace-self-host") {
		t.Fatalf("request path = %q, want path containing trace-self-host", req.Path)
	}
	if got := req.Header.Get("X-Api-Key"); got != "integration-api-key" {
		t.Fatalf("X-Api-Key = %q, want %q", got, "integration-api-key")
	}
	if got := stdout.String(); !strings.Contains(got, `"from self host"`) {
		t.Fatalf("stdout = %q, want self-host response content", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
