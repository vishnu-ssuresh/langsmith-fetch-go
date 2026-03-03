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

	langsmith "langsmith-sdk/go/langsmith"

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

func TestExecute_Threads_Integration(t *testing.T) {
	t.Parallel()

	requestCh := make(chan capturedRequest, 8)
	server := newMockLangSmithServer(t, func(w http.ResponseWriter, req capturedRequest) {
		requestCh <- req
		w.Header().Set("Content-Type", "application/json")

		switch req.Path {
		case "/runs/query":
			_, _ = io.WriteString(
				w,
				`{"runs":[{"id":"run-1","name":"one","start_time":"2026-01-01T00:00:00Z","extra":{"metadata":{"thread_id":"thread-a"}}},{"id":"run-2","name":"two","start_time":"2026-01-01T00:01:00Z","extra":{"metadata":{"thread_id":"thread-b"}}},{"id":"run-3","name":"three","start_time":"2026-01-01T00:02:00Z","extra":{"metadata":{"thread_id":"thread-a"}}}]}`,
			)
		case "/runs/threads/thread-a":
			_, _ = io.WriteString(
				w,
				`{"previews":{"all_messages":"{\"role\":\"user\",\"content\":\"hello from a\"}"}}`,
			)
		case "/runs/threads/thread-b":
			_, _ = io.WriteString(
				w,
				`{"previews":{"all_messages":"{\"role\":\"assistant\",\"content\":\"hello from b\"}"}}`,
			)
		default:
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

	requests := make([]capturedRequest, 0, 3)
	for i := 0; i < 3; i++ {
		select {
		case req := <-requestCh:
			requests = append(requests, req)
		default:
			t.Fatalf("request %d missing; got %d request(s)", i+1, len(requests))
		}
	}
	select {
	case extra := <-requestCh:
		t.Fatalf("unexpected extra request: %s %s", extra.Method, extra.Path)
	default:
	}

	var queryRequest capturedRequest
	hasQuery := false
	threadPaths := make(map[string]capturedRequest, 2)
	for _, req := range requests {
		if req.Path == "/runs/query" {
			queryRequest = req
			hasQuery = true
			continue
		}
		threadPaths[req.Path] = req
	}

	if !hasQuery {
		t.Fatalf("missing /runs/query request in %#v", requests)
	}
	if queryRequest.Method != http.MethodPost {
		t.Fatalf("query method = %q, want %q", queryRequest.Method, http.MethodPost)
	}
	if got := queryRequest.Header.Get("X-API-Key"); got != "integration-api-key" {
		t.Fatalf("query X-API-Key = %q, want %q", got, "integration-api-key")
	}

	var body struct {
		Session []string `json:"session"`
		IsRoot  bool     `json:"is_root"`
		Limit   int      `json:"limit"`
	}
	if err := json.Unmarshal(queryRequest.Body, &body); err != nil {
		t.Fatalf("decode query body: %v", err)
	}
	if len(body.Session) != 1 || body.Session[0] != "project-123" {
		t.Fatalf("query session = %#v, want [project-123]", body.Session)
	}
	if !body.IsRoot {
		t.Fatalf("query is_root = %v, want true", body.IsRoot)
	}
	if body.Limit <= 0 {
		t.Fatalf("query limit = %d, want > 0", body.Limit)
	}

	if len(threadPaths) != 2 {
		t.Fatalf("thread request count = %d, want 2", len(threadPaths))
	}
	for _, path := range []string{"/runs/threads/thread-a", "/runs/threads/thread-b"} {
		req, ok := threadPaths[path]
		if !ok {
			t.Fatalf("missing thread request %q", path)
		}
		if req.Method != http.MethodGet {
			t.Fatalf("thread method for %s = %q, want %q", path, req.Method, http.MethodGet)
		}
		if !strings.Contains(req.Query, "select=all_messages") {
			t.Fatalf("thread query for %s = %q, want select=all_messages", path, req.Query)
		}
		if !strings.Contains(req.Query, "session_id=project-123") {
			t.Fatalf("thread query for %s = %q, want session_id=project-123", path, req.Query)
		}
		if got := req.Header.Get("X-API-Key"); got != "integration-api-key" {
			t.Fatalf("thread X-API-Key for %s = %q, want %q", path, got, "integration-api-key")
		}
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
	if !strings.Contains(output, "project_name: demo-project") {
		t.Fatalf("stdout = %q, want project_name", output)
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

func TestExecute_Trace_Integration_RetriesOnServerError(t *testing.T) {
	t.Parallel()

	var attempt atomic.Int64
	server := newMockLangSmithServer(t, func(w http.ResponseWriter, req capturedRequest) {
		current := attempt.Add(1)
		w.Header().Set("Content-Type", "application/json")

		if current == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"transient"}`)
			return
		}
		_, _ = io.WriteString(
			w,
			`{"id":"trace-123","messages":[{"role":"assistant","content":"ok"}]}`,
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
	if got := attempt.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
	if got := stdout.String(); !strings.Contains(got, `"role": "assistant"`) {
		t.Fatalf("stdout = %q, want JSON trace output", got)
	}
}

func TestExecute_Trace_Integration_SurfacesTypedUnauthorizedError(t *testing.T) {
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
	if got := attempt.Load(); got != 1 {
		t.Fatalf("attempts = %d, want 1", got)
	}
	if !errors.Is(err, langsmith.ErrUnauthorized) {
		t.Fatalf("Execute() error = %v, want errors.Is(_, ErrUnauthorized)", err)
	}
}

func TestExecute_Trace_Integration_NetworkFailure(t *testing.T) {
	t.Parallel()

	deps := NewDeps()
	deps.LoadConfig = func() config.Values {
		return config.Values{
			APIKey:   "integration-api-key",
			Endpoint: "http://127.0.0.1:1",
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
	if !strings.Contains(err.Error(), "execute request") {
		t.Fatalf("Execute() error = %v, want network execute-request error", err)
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
	if req.Path != "/runs/trace-self-host" {
		t.Fatalf("request path = %q, want %q", req.Path, "/runs/trace-self-host")
	}
	if got := req.Header.Get("X-API-Key"); got != "integration-api-key" {
		t.Fatalf("X-API-Key = %q, want %q", got, "integration-api-key")
	}
	if got := stdout.String(); !strings.Contains(got, `"from self host"`) {
		t.Fatalf("stdout = %q, want self-host response content", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
