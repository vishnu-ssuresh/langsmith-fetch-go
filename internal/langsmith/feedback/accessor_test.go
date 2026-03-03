// accessor_test.go validates feedback accessor request and response behavior.
package feedback

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

func TestListByRuns_RequiresRunIDs(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.ListByRuns(context.Background(), ListParams{})
	if err == nil || !strings.Contains(err.Error(), "at least one run id is required") {
		t.Fatalf("ListByRuns() error = %v, want run id required", err)
	}
	if doer.called {
		t.Fatal("Do() called unexpectedly")
	}
}

func TestListByRuns_BuildsRequestAndParsesArrayResponse(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body: []byte(`[
  {"id":"fb-1","run_id":"run-1","key":"correctness","score":1,"value":"good","comment":"ok"}
]`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	items, err := accessor.ListByRuns(context.Background(), ListParams{
		RunIDs: []string{"run-1", "run-2"},
		Limit:  5,
		Keys:   []string{"correctness"},
		Source: []string{"api"},
	})
	if err != nil {
		t.Fatalf("ListByRuns() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != "fb-1" {
		t.Fatalf("items = %+v, want one item fb-1", items)
	}
	if doer.req.Method != http.MethodGet {
		t.Fatalf("Method = %q, want GET", doer.req.Method)
	}
	if doer.req.Path != "/feedback" {
		t.Fatalf("Path = %q, want %q", doer.req.Path, "/feedback")
	}

	wantQuery := url.Values{
		"run":    []string{"run-1", "run-2"},
		"limit":  []string{"5"},
		"key":    []string{"correctness"},
		"source": []string{"api"},
	}
	if got := doer.req.Query.Encode(); got != wantQuery.Encode() {
		t.Fatalf("Query = %q, want %q", got, wantQuery.Encode())
	}
}

func TestListByRuns_DefaultAndMaxLimit(t *testing.T) {
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

	_, err = accessor.ListByRuns(context.Background(), ListParams{
		RunIDs: []string{"run-1"},
		Limit:  9999,
	})
	if err != nil {
		t.Fatalf("ListByRuns() error = %v", err)
	}
	if got := doer.req.Query.Get("limit"); got != "100" {
		t.Fatalf("limit = %q, want %q", got, "100")
	}
}

func TestListByRuns_ParsesWrappedResponse(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"items":[{"id":"fb-2","run_id":"run-1","key":"k"}]}`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	items, err := accessor.ListByRuns(context.Background(), ListParams{
		RunIDs: []string{"run-1"},
	})
	if err != nil {
		t.Fatalf("ListByRuns() error = %v", err)
	}
	if len(items) != 1 || items[0].ID != "fb-2" {
		t.Fatalf("items = %+v, want one item fb-2", items)
	}
}

func TestListByRuns_PropagatesDoError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{err: errors.New("network failed")}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.ListByRuns(context.Background(), ListParams{
		RunIDs: []string{"run-1"},
	})
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("ListByRuns() error = %v, want wrapped do error", err)
	}
}

func TestListByRuns_StatusError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusBadRequest,
			Body:       []byte("bad request"),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.ListByRuns(context.Background(), ListParams{
		RunIDs: []string{"run-1"},
	})
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("ListByRuns() error = %v, want status error", err)
	}
}

func TestListByRuns_StatusErrorMapsTypedErrors(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       []byte("rate limited"),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.ListByRuns(context.Background(), ListParams{
		RunIDs: []string{"run-1"},
	})
	if err == nil {
		t.Fatal("ListByRuns() error = nil, want non-nil")
	}
	if !errors.Is(err, langsmith.ErrRateLimited) {
		t.Fatalf("ListByRuns() error = %v, want errors.Is(_, ErrRateLimited)", err)
	}
}

func TestListByRuns_DecodeError(t *testing.T) {
	t.Parallel()

	doer := &fakeDoer{
		resp: transport.Response{
			StatusCode: http.StatusOK,
			Body:       []byte(`{"items":`),
		},
	}
	accessor, err := NewAccessor(doer)
	if err != nil {
		t.Fatalf("NewAccessor() error = %v", err)
	}

	_, err = accessor.ListByRuns(context.Background(), ListParams{
		RunIDs: []string{"run-1"},
	})
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("ListByRuns() error = %v, want decode error", err)
	}
}
