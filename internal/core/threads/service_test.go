// service_test.go validates bulk thread list orchestration behavior.
package threads

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

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
	mu          sync.Mutex
	calls       []langsmiththreads.GetMessagesParams
	data        map[string][]langsmiththreads.Message
	err         map[string]error
	delay       time.Duration
	inFlight    int
	maxInFlight int
}

func (f *fakeListThreadsAccessor) GetMessages(_ context.Context, params langsmiththreads.GetMessagesParams) ([]langsmiththreads.Message, error) {
	f.mu.Lock()
	f.calls = append(f.calls, params)
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
	callErr := error(nil)
	if f.err != nil {
		if err, ok := f.err[params.ThreadID]; ok {
			callErr = err
		}
	}
	data := map[string][]langsmiththreads.Message(nil)
	if f.data != nil {
		data = f.data
	}
	delay := f.delay
	f.mu.Unlock()

	if delay > 0 {
		time.Sleep(delay)
	}

	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()

	if callErr != nil {
		return nil, callErr
	}
	if data == nil {
		return nil, nil
	}
	return data[params.ThreadID], nil
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
		StartTime: "2025-12-09T10:00:00Z",
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
	if runs.params.StartTime != "2025-12-09T10:00:00Z" {
		t.Fatalf("StartTime = %q, want %q", runs.params.StartTime, "2025-12-09T10:00:00Z")
	}
	if len(threads.calls) != 2 {
		t.Fatalf("len(calls) = %d, want 2", len(threads.calls))
	}
	gotCalledIDs := []string{threads.calls[0].ThreadID, threads.calls[1].ThreadID}
	slices.Sort(gotCalledIDs)
	wantCalledIDs := []string{"thread-1", "thread-2"}
	if !slices.Equal(gotCalledIDs, wantCalledIDs) {
		t.Fatalf("thread calls = %v, want %v", gotCalledIDs, wantCalledIDs)
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

func TestList_RespectsMaxConcurrent(t *testing.T) {
	t.Parallel()

	runs := &fakeListRunsAccessor{
		runs: []langsmithruns.RootRun{
			{ThreadID: "thread-1"},
			{ThreadID: "thread-2"},
			{ThreadID: "thread-3"},
		},
	}
	threads := &fakeListThreadsAccessor{
		data: map[string][]langsmiththreads.Message{
			"thread-1": {[]byte(`{"role":"user","content":"a"}`)},
			"thread-2": {[]byte(`{"role":"user","content":"b"}`)},
			"thread-3": {[]byte(`{"role":"user","content":"c"}`)},
		},
		delay: 20 * time.Millisecond,
	}
	lister, err := NewLister(runs, threads)
	if err != nil {
		t.Fatalf("NewLister() error = %v", err)
	}

	_, err = lister.List(context.Background(), ListParams{
		ProjectID:     "project-123",
		Limit:         3,
		MaxConcurrent: 1,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if threads.maxInFlight > 1 {
		t.Fatalf("maxInFlight = %d, want <= 1", threads.maxInFlight)
	}
}

func TestList_ReportsProgress(t *testing.T) {
	t.Parallel()

	runs := &fakeListRunsAccessor{
		runs: []langsmithruns.RootRun{
			{ThreadID: "thread-1"},
			{ThreadID: "thread-2"},
		},
	}
	threads := &fakeListThreadsAccessor{
		data: map[string][]langsmiththreads.Message{
			"thread-1": {[]byte(`{"role":"user","content":"a"}`)},
			"thread-2": {[]byte(`{"role":"user","content":"b"}`)},
		},
	}
	lister, err := NewLister(runs, threads)
	if err != nil {
		t.Fatalf("NewLister() error = %v", err)
	}

	var mu sync.Mutex
	var updates [][2]int
	_, err = lister.List(context.Background(), ListParams{
		ProjectID:    "project-123",
		Limit:        2,
		ShowProgress: true,
		Progress: func(completed int, total int) {
			mu.Lock()
			updates = append(updates, [2]int{completed, total})
			mu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(updates) < 2 {
		t.Fatalf("updates = %v, want at least start and finish", updates)
	}
	first := updates[0]
	last := updates[len(updates)-1]
	if first != [2]int{0, 2} {
		t.Fatalf("first update = %v, want [0 2]", first)
	}
	if last != [2]int{2, 2} {
		t.Fatalf("last update = %v, want [2 2]", last)
	}
}
