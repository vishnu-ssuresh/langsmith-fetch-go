// accessor_test.go validates project accessor request and response behavior.
package projects

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	langsmith "langsmith-sdk/go/langsmith"
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

func TestResolveProjectUUID_RequiresProjectName(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.ResolveProjectUUID(context.Background(), "   ")
	if err == nil || !strings.Contains(err.Error(), "project name is required") {
		t.Fatalf("ResolveProjectUUID() error = %v, want project name required", err)
	}
	if doer.called {
		t.Fatal("Do() called unexpectedly")
	}
}

func TestResolveProjectUUID_BuildsRequestAndParsesArrayResponse(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`[{"id":"proj-123","name":"my-project"}]`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	projectID, err := accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err != nil {
		t.Fatalf("ResolveProjectUUID() error = %v", err)
	}
	if projectID != "proj-123" {
		t.Fatalf("projectID = %q, want %q", projectID, "proj-123")
	}

	if doer.req.Method != http.MethodGet {
		t.Fatalf("Method = %q, want GET", doer.req.Method)
	}
	if doer.req.Path != "/sessions" {
		t.Fatalf("Path = %q, want %q", doer.req.Path, "/sessions")
	}
	wantQuery := url.Values{
		"name":          []string{"my-project"},
		"limit":         []string{"1"},
		"include_stats": []string{"false"},
	}
	if got := doer.req.Query.Encode(); got != wantQuery.Encode() {
		t.Fatalf("Query = %q, want %q", got, wantQuery.Encode())
	}
}

func TestResolveProjectUUID_ParsesObjectListResponse(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"sessions":[{"id":"proj-456","name":"my-project"}]}`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	projectID, err := accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err != nil {
		t.Fatalf("ResolveProjectUUID() error = %v", err)
	}
	if projectID != "proj-456" {
		t.Fatalf("projectID = %q, want %q", projectID, "proj-456")
	}
}

func TestResolveProjectUUID_ParsesSingleObjectResponse(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"id":"proj-789","name":"my-project"}`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	projectID, err := accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err != nil {
		t.Fatalf("ResolveProjectUUID() error = %v", err)
	}
	if projectID != "proj-789" {
		t.Fatalf("projectID = %q, want %q", projectID, "proj-789")
	}
}

func TestResolveProjectUUID_PropagatesDoError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{err: errors.New("network failed")}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("ResolveProjectUUID() error = %v, want wrapped do error", err)
	}
}

func TestResolveProjectUUID_StatusError(t *testing.T) {
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

	_, err = accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err == nil || !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("ResolveProjectUUID() error = %v, want status error", err)
	}
}

func TestResolveProjectUUID_StatusErrorMapsTypedErrors(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusForbidden,
			Body:       []byte("forbidden"),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err == nil {
		t.Fatal("ResolveProjectUUID() error = nil, want non-nil")
	}
	if !errors.Is(err, langsmith.ErrForbidden) {
		t.Fatalf("ResolveProjectUUID() error = %v, want errors.Is(_, ErrForbidden)", err)
	}
}

func TestResolveProjectUUID_DecodeError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"sessions":`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("ResolveProjectUUID() error = %v, want decode error", err)
	}
}

func TestResolveProjectUUID_NotFound(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`[]`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.ResolveProjectUUID(context.Background(), "my-project")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("ResolveProjectUUID() error = %v, want not found error", err)
	}
}
