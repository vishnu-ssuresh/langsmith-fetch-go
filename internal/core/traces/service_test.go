// service_test.go validates trace service requests and response handling.
package traces

import (
	"context"
	"encoding/json"
	"errors"
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

func TestNew_RequiresDoer(t *testing.T) {
	t.Parallel()

	svc, err := New(nil)
	if err == nil {
		t.Fatal("New(nil) error = nil, want non-nil")
	}
	if svc != nil {
		t.Fatal("New(nil) service != nil, want nil")
	}
}

func TestList_RequiresProjectID(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{}
	svc, err := New(doer)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{})
	if err == nil || !strings.Contains(err.Error(), "project id is required") {
		t.Fatalf("List() error = %v, want project id required", err)
	}
	if doer.called {
		t.Fatal("Do() called unexpectedly")
	}
}

func TestList_DefaultLimitAndDecode(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: 200,
			Body:       []byte(`{"runs":[{"id":"run-1","name":"trace-a","start_time":"2026-01-01T00:00:00Z"}]}`),
		},
	}
	svc, err := New(doer)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	runs, err := svc.List(context.Background(), ListParams{ProjectID: "project-123"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
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

func TestList_UsesExplicitLimit(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: 200,
			Body:       []byte(`{"runs":[]}`),
		},
	}
	svc, err := New(doer)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{
		ProjectID: "project-123",
		Limit:     5,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	var body queryRunsRequest
	if err := json.Unmarshal(doer.req.Body, &body); err != nil {
		t.Fatalf("json.Unmarshal(request body) error = %v", err)
	}
	if body.Limit != 5 {
		t.Fatalf("Limit = %d, want 5", body.Limit)
	}
}

func TestList_PropagatesDoError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{err: errors.New("network failed")}
	svc, err := New(doer)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("List() error = %v, want wrapped do error", err)
	}
}

func TestList_StatusError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: 400,
			Body:       []byte(`bad request`),
		},
	}
	svc, err := New(doer)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("List() error = %v, want status error", err)
	}
}

func TestList_DecodeError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: 200,
			Body:       []byte(`{"runs":`),
		},
	}
	svc, err := New(doer)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("List() error = %v, want decode error", err)
	}
}
