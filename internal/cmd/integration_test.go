// integration_test.go adds end-to-end command tests against a mock HTTP API.
package cmd

import (
	"bytes"
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
