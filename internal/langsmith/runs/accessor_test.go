// accessor_test.go validates run accessor request and response behavior.
package runs

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

func TestQueryRoot_RequiresProjectID(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.QueryRoot(context.Background(), QueryRootParams{})
	if err == nil || !strings.Contains(err.Error(), "project id is required") {
		t.Fatalf("QueryRoot() error = %v, want project id required", err)
	}
	if doer.called {
		t.Fatal("Do() called unexpectedly")
	}
}

func TestQueryRoot_DefaultLimitAndDecode(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: 200,
			Body:       []byte(`{"runs":[{"id":"run-1","name":"trace-a","start_time":"2026-01-01T00:00:00Z"}]}`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	runs, err := accessor.QueryRoot(context.Background(), QueryRootParams{ProjectID: "project-123"})
	if err != nil {
		t.Fatalf("QueryRoot() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("len(runs) = %d, want 1", len(runs))
	}
	if runs[0].ID != "run-1" {
		t.Fatalf("runs[0].ID = %q, want %q", runs[0].ID, "run-1")
	}

	if doer.req.Method != "POST" {
		t.Fatalf("Method = %q, want POST", doer.req.Method)
	}
	if doer.req.Path != "/runs/query" {
		t.Fatalf("Path = %q, want %q", doer.req.Path, "/runs/query")
	}

	var body queryRunsRequest
	if err := json.Unmarshal(doer.req.Body, &body); err != nil {
		t.Fatalf("json.Unmarshal(request body) error = %v", err)
	}
	if len(body.Session) != 1 || body.Session[0] != "project-123" {
		t.Fatalf("Session = %#v, want []string{\"project-123\"}", body.Session)
	}
	if !body.IsRoot {
		t.Fatalf("IsRoot = %v, want true", body.IsRoot)
	}
	if body.Limit != 20 {
		t.Fatalf("Limit = %d, want 20", body.Limit)
	}
}

func TestQueryRoot_UsesExplicitLimit(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: 200,
			Body:       []byte(`{"runs":[]}`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.QueryRoot(context.Background(), QueryRootParams{
		ProjectID: "project-123",
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("QueryRoot() error = %v", err)
	}

	var body queryRunsRequest
	if err := json.Unmarshal(doer.req.Body, &body); err != nil {
		t.Fatalf("json.Unmarshal(request body) error = %v", err)
	}
	if body.Limit != 5 {
		t.Fatalf("Limit = %d, want 5", body.Limit)
	}
}

func TestQueryRoot_PropagatesDoError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{err: errors.New("network failed")}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.QueryRoot(context.Background(), QueryRootParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("QueryRoot() error = %v, want wrapped do error", err)
	}
}

func TestQueryRoot_StatusError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: 400,
			Body:       []byte("bad request"),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.QueryRoot(context.Background(), QueryRootParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("QueryRoot() error = %v, want status error", err)
	}
}

func TestQueryRoot_DecodeError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: 200,
			Body:       []byte(`{"runs":`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.QueryRoot(context.Background(), QueryRootParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("QueryRoot() error = %v, want decode error", err)
	}
}

func TestGetRun_RequiresRunID(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetRun(context.Background(), GetRunParams{})
	if err == nil || !strings.Contains(err.Error(), "run id is required") {
		t.Fatalf("GetRun() error = %v, want run id required", err)
	}
	if doer.called {
		t.Fatal("Do() called unexpectedly")
	}
}

func TestGetRun_BuildsRequestAndDecodesResponse(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body: []byte(`{
  "id":"trace-1",
  "messages":[{"role":"user","content":"hi"}],
  "outputs":{"messages":[{"role":"assistant","content":"hello"}]}
}`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	run, err := accessor.GetRun(context.Background(), GetRunParams{
		RunID:           "trace/a b",
		IncludeMessages: true,
	})
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if run.ID != "trace-1" {
		t.Fatalf("run.ID = %q, want %q", run.ID, "trace-1")
	}
	if len(run.Messages) != 1 {
		t.Fatalf("len(run.Messages) = %d, want 1", len(run.Messages))
	}
	if len(run.Outputs.Messages) != 1 {
		t.Fatalf("len(run.Outputs.Messages) = %d, want 1", len(run.Outputs.Messages))
	}
	if doer.req.Method != http.MethodGet {
		t.Fatalf("Method = %q, want GET", doer.req.Method)
	}
	if doer.req.Path != "/runs/trace%2Fa%20b" {
		t.Fatalf("Path = %q, want escaped path", doer.req.Path)
	}
	wantQuery := url.Values{"include_messages": []string{"true"}}
	if got := doer.req.Query.Encode(); got != wantQuery.Encode() {
		t.Fatalf("Query = %q, want %q", got, wantQuery.Encode())
	}
}

func TestGetRun_PropagatesDoError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{err: errors.New("network failed")}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetRun(context.Background(), GetRunParams{RunID: "trace-1"})
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("GetRun() error = %v, want wrapped do error", err)
	}
}

func TestGetRun_StatusError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusNotFound,
			Body:       []byte("not found"),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetRun(context.Background(), GetRunParams{RunID: "trace-1"})
	if err == nil || !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("GetRun() error = %v, want status error", err)
	}
}

func TestGetRun_DecodeError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"id":`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.GetRun(context.Background(), GetRunParams{RunID: "trace-1"})
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("GetRun() error = %v, want decode error", err)
	}
}
