// service_test.go validates bulk thread list orchestration behavior.
package threads

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
	langsmiththreads "langsmith-fetch-go/internal/langsmith/threads"
)

type fakeListRunsAccessor struct {
	params langsmithruns.QueryRootParams
	runs   []langsmithruns.RootRun
	err    error
	called bool
}

func (f *fakeListRunsAccessor) QueryRootRuns(_ context.Context, params langsmithruns.QueryRootParams) ([]langsmithruns.RootRun, error) {
	f.called = true
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return f.runs, nil
}

type fakeListThreadsAccessor struct {
	calls []langsmiththreads.GetMessagesParams
	data  map[string][]langsmiththreads.Message
	err   map[string]error
}

func (f *fakeListThreadsAccessor) GetMessages(_ context.Context, params langsmiththreads.GetMessagesParams) ([]langsmiththreads.Message, error) {
	f.calls = append(f.calls, params)
	if f.err != nil {
		if callErr, ok := f.err[params.ThreadID]; ok {
			return nil, callErr
		}
	}
	if f.data == nil {
		return nil, nil
	}
	return f.data[params.ThreadID], nil
}

func TestNewLister_RequiresAccessors(t *testing.T) {
	t.Parallel()

	lister, err := NewLister(nil, nil)
	if err == nil {
		t.Fatal("NewLister(nil,nil) error = nil, want non-nil")
	}
	if lister != nil {
		t.Fatal("NewLister(nil,nil) lister != nil, want nil")
	}
}

func TestList_RequiresProjectID(t *testing.T) {
	t.Parallel()

	runs := &fakeListRunsAccessor{}
	threads := &fakeListThreadsAccessor{}
	lister, err := NewLister(runs, threads)
	if err != nil {
		t.Fatalf("NewLister() error = %v", err)
	}

	_, err = lister.List(context.Background(), ListParams{})
	if err == nil || !strings.Contains(err.Error(), "project id is required") {
		t.Fatalf("List() error = %v, want project id required", err)
	}
	if runs.called {
		t.Fatal("QueryRootRuns() called unexpectedly")
	}
}

func TestList_DedupesOrdersAndLimits(t *testing.T) {
	t.Parallel()

	runs := &fakeListRunsAccessor{
		runs: []langsmithruns.RootRun{
			{ThreadID: "thread-1"},
			{ThreadID: "thread-2"},
			{ThreadID: "thread-1"},
			{ThreadID: "thread-3"},
		},
	}
	threads := &fakeListThreadsAccessor{
		data: map[string][]langsmiththreads.Message{
			"thread-1": {[]byte(`{"role":"user","content":"a"}`)},
			"thread-2": {[]byte(`{"role":"user","content":"b"}`)},
			"thread-3": {[]byte(`{"role":"user","content":"c"}`)},
		},
	}
	lister, err := NewLister(runs, threads)
	if err != nil {
		t.Fatalf("NewLister() error = %v", err)
	}

	out, err := lister.List(context.Background(), ListParams{
		ProjectID: "project-123",
		Limit:     2,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[0].ThreadID != "thread-1" || out[1].ThreadID != "thread-2" {
		t.Fatalf("thread order = [%s, %s], want [thread-1, thread-2]", out[0].ThreadID, out[1].ThreadID)
	}
	if runs.params.ProjectID != "project-123" {
		t.Fatalf("ProjectID = %q, want %q", runs.params.ProjectID, "project-123")
	}
	if runs.params.Limit < threadListMinQuery {
		t.Fatalf("Query limit = %d, want at least %d", runs.params.Limit, threadListMinQuery)
	}
	if len(threads.calls) != 2 {
		t.Fatalf("len(calls) = %d, want 2", len(threads.calls))
	}
	gotIDs := []string{threads.calls[0].ThreadID, threads.calls[1].ThreadID}
	wantIDs := []string{"thread-1", "thread-2"}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("thread calls = %v, want %v", gotIDs, wantIDs)
	}
}

func TestList_PropagatesRunQueryError(t *testing.T) {
	t.Parallel()

	runs := &fakeListRunsAccessor{err: errors.New("run query failed")}
	threads := &fakeListThreadsAccessor{}
	lister, err := NewLister(runs, threads)
	if err != nil {
		t.Fatalf("NewLister() error = %v", err)
	}

	_, err = lister.List(context.Background(), ListParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "run query failed") {
		t.Fatalf("List() error = %v, want wrapped run query error", err)
	}
}

func TestList_PropagatesThreadFetchError(t *testing.T) {
	t.Parallel()

	runs := &fakeListRunsAccessor{
		runs: []langsmithruns.RootRun{{ThreadID: "thread-1"}},
	}
	threads := &fakeListThreadsAccessor{
		err: map[string]error{
			"thread-1": errors.New("thread fetch failed"),
		},
	}
	lister, err := NewLister(runs, threads)
	if err != nil {
		t.Fatalf("NewLister() error = %v", err)
	}

	_, err = lister.List(context.Background(), ListParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "thread fetch failed") {
		t.Fatalf("List() error = %v, want wrapped thread fetch error", err)
	}
}

func TestList_SkipsRunsWithoutThreadID(t *testing.T) {
	t.Parallel()

	runs := &fakeListRunsAccessor{
		runs: []langsmithruns.RootRun{
			{ThreadID: ""},
			{ThreadID: "thread-1"},
		},
	}
	threads := &fakeListThreadsAccessor{
		data: map[string][]langsmiththreads.Message{
			"thread-1": {[]byte(`{"role":"assistant","content":"ok"}`)},
		},
	}
	lister, err := NewLister(runs, threads)
	if err != nil {
		t.Fatalf("NewLister() error = %v", err)
	}

	out, err := lister.List(context.Background(), ListParams{ProjectID: "project-123"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(out) != 1 || out[0].ThreadID != "thread-1" {
		t.Fatalf("out = %+v, want only thread-1", out)
	}
}
