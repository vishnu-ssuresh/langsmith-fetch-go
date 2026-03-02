// service_test.go validates trace service orchestration behavior.
package traces

import (
	"context"
	"errors"
	"strings"
	"testing"

	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
)

type fakeRunsAccessor struct {
	params langsmithruns.QueryRootParams
	runs   []langsmithruns.Summary
	err    error
	called bool
}

func (f *fakeRunsAccessor) QueryRoot(_ context.Context, params langsmithruns.QueryRootParams) ([]langsmithruns.Summary, error) {
	f.called = true
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return f.runs, nil
}

func TestNew_RequiresRunsAccessor(t *testing.T) {
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

	accessor := &fakeRunsAccessor{}
	svc, err := New(accessor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{})
	if err == nil || !strings.Contains(err.Error(), "project id is required") {
		t.Fatalf("List() error = %v, want project id required", err)
	}
	if accessor.called {
		t.Fatal("QueryRoot() called unexpectedly")
	}
}

func TestList_DefaultLimitAndReturn(t *testing.T) {
	t.Parallel()

	accessor := &fakeRunsAccessor{
		runs: []langsmithruns.Summary{
			{ID: "run-1", Name: "trace-a", StartTime: "2026-01-01T00:00:00Z"},
		},
	}
	svc, err := New(accessor)
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
	if accessor.params.ProjectID != "project-123" {
		t.Fatalf("ProjectID = %q, want %q", accessor.params.ProjectID, "project-123")
	}
	if accessor.params.Limit != 20 {
		t.Fatalf("Limit = %d, want 20", accessor.params.Limit)
	}
}

func TestList_UsesExplicitLimit(t *testing.T) {
	t.Parallel()

	accessor := &fakeRunsAccessor{}
	svc, err := New(accessor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{
		ProjectID: "project-123",
		Limit:     5,
		StartTime: "2025-12-09T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if accessor.params.Limit != 5 {
		t.Fatalf("Limit = %d, want 5", accessor.params.Limit)
	}
	if accessor.params.StartTime != "2025-12-09T10:00:00Z" {
		t.Fatalf("StartTime = %q, want %q", accessor.params.StartTime, "2025-12-09T10:00:00Z")
	}
}

func TestList_PropagatesAccessorError(t *testing.T) {
	t.Parallel()

	accessor := &fakeRunsAccessor{err: errors.New("network failed")}
	svc, err := New(accessor)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("List() error = %v, want wrapped do error", err)
	}
}
