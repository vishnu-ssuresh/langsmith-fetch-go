// service_test.go validates trace service orchestration behavior.
package traces

import (
	"context"
	"errors"
	"strings"
	"testing"

	langsmithfeedback "langsmith-fetch-go/internal/langsmith/feedback"
	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
)

type fakeRunsAccessor struct {
	queryParams langsmithruns.QueryRootParams
	queryRuns   []langsmithruns.Summary
	queryErr    error
	queryCalled bool

	getParams []langsmithruns.GetRunParams
	getByID   map[string]langsmithruns.Run
	getErr    map[string]error
}

func (f *fakeRunsAccessor) QueryRoot(_ context.Context, params langsmithruns.QueryRootParams) ([]langsmithruns.Summary, error) {
	f.queryCalled = true
	f.queryParams = params
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.queryRuns, nil
}

func (f *fakeRunsAccessor) GetRun(_ context.Context, params langsmithruns.GetRunParams) (langsmithruns.Run, error) {
	f.getParams = append(f.getParams, params)
	if f.getErr != nil {
		if err, ok := f.getErr[params.RunID]; ok {
			return langsmithruns.Run{}, err
		}
	}
	if f.getByID == nil {
		return langsmithruns.Run{}, nil
	}
	return f.getByID[params.RunID], nil
}

type fakeFeedbackAccessor struct {
	calls []langsmithfeedback.ListParams
	byRun map[string][]langsmithfeedback.Item
	err   map[string]error
}

func (f *fakeFeedbackAccessor) ListByRuns(_ context.Context, params langsmithfeedback.ListParams) ([]langsmithfeedback.Item, error) {
	f.calls = append(f.calls, params)
	if len(params.RunIDs) == 0 {
		return nil, nil
	}
	runID := params.RunIDs[0]
	if f.err != nil {
		if err, ok := f.err[runID]; ok {
			return nil, err
		}
	}
	if f.byRun == nil {
		return nil, nil
	}
	return f.byRun[runID], nil
}

func TestNew_RequiresRunsAccessor(t *testing.T) {
	t.Parallel()

	svc, err := New(nil, nil)
	if err == nil {
		t.Fatal("New(nil,nil) error = nil, want non-nil")
	}
	if svc != nil {
		t.Fatal("New(nil,nil) service != nil, want nil")
	}
}

func TestList_RequiresProjectID(t *testing.T) {
	t.Parallel()

	runs := &fakeRunsAccessor{}
	svc, err := New(runs, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{})
	if err == nil || !strings.Contains(err.Error(), "project id is required") {
		t.Fatalf("List() error = %v, want project id required", err)
	}
	if runs.queryCalled {
		t.Fatal("QueryRoot() called unexpectedly")
	}
}

func TestList_DefaultLimitAndReturn(t *testing.T) {
	t.Parallel()

	runs := &fakeRunsAccessor{
		queryRuns: []langsmithruns.Summary{
			{ID: "run-1", Name: "trace-a", StartTime: "2026-01-01T00:00:00Z"},
		},
	}
	svc, err := New(runs, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	out, err := svc.List(context.Background(), ListParams{ProjectID: "project-123"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].ID != "run-1" {
		t.Fatalf("out[0].ID = %q, want %q", out[0].ID, "run-1")
	}
	if out[0].Metadata != nil {
		t.Fatalf("Metadata = %+v, want nil", out[0].Metadata)
	}
	if runs.queryParams.ProjectID != "project-123" {
		t.Fatalf("ProjectID = %q, want %q", runs.queryParams.ProjectID, "project-123")
	}
	if runs.queryParams.Limit != 20 {
		t.Fatalf("Limit = %d, want 20", runs.queryParams.Limit)
	}
	if len(runs.getParams) != 0 {
		t.Fatalf("GetRun calls = %d, want 0", len(runs.getParams))
	}
}

func TestList_UsesExplicitLimitAndStartTime(t *testing.T) {
	t.Parallel()

	runs := &fakeRunsAccessor{}
	svc, err := New(runs, nil)
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
	if runs.queryParams.Limit != 5 {
		t.Fatalf("Limit = %d, want 5", runs.queryParams.Limit)
	}
	if runs.queryParams.StartTime != "2025-12-09T10:00:00Z" {
		t.Fatalf("StartTime = %q, want %q", runs.queryParams.StartTime, "2025-12-09T10:00:00Z")
	}
}

func TestList_PropagatesRunQueryError(t *testing.T) {
	t.Parallel()

	runs := &fakeRunsAccessor{queryErr: errors.New("network failed")}
	svc, err := New(runs, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{ProjectID: "project-123"})
	if err == nil || !strings.Contains(err.Error(), "network failed") {
		t.Fatalf("List() error = %v, want wrapped query error", err)
	}
}

func TestList_IncludeMetadataFetchesRun(t *testing.T) {
	t.Parallel()

	promptTokens := 42
	totalCost := 1.25
	runs := &fakeRunsAccessor{
		queryRuns: []langsmithruns.Summary{
			{ID: "run-1", Name: "trace-a", StartTime: "2026-01-01T00:00:00Z"},
		},
		getByID: map[string]langsmithruns.Run{
			"run-1": {
				ID:             "run-1",
				Status:         "completed",
				StartTime:      "2026-01-01T00:00:00Z",
				EndTime:        "2026-01-01T00:00:02Z",
				PromptTokens:   &promptTokens,
				TotalCost:      &totalCost,
				FirstTokenTime: "2026-01-01T00:00:00.500Z",
				Extra: langsmithruns.Extra{
					Metadata: []byte(`{"thread_id":"thread-1"}`),
				},
				FeedbackStats: []byte(`{"correctness":1}`),
			},
		},
	}
	svc, err := New(runs, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	out, err := svc.List(context.Background(), ListParams{
		ProjectID:       "project-123",
		IncludeMetadata: true,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(runs.getParams) != 1 || runs.getParams[0].RunID != "run-1" {
		t.Fatalf("GetRun params = %+v, want one call for run-1", runs.getParams)
	}
	if out[0].Metadata == nil {
		t.Fatal("Metadata = nil, want non-nil")
	}
	if out[0].Metadata.Status != "completed" {
		t.Fatalf("status = %q, want %q", out[0].Metadata.Status, "completed")
	}
	if out[0].Metadata.DurationMS == nil || *out[0].Metadata.DurationMS != 2000 {
		t.Fatalf("duration_ms = %+v, want 2000", out[0].Metadata.DurationMS)
	}
	if out[0].Metadata.TokenUsage.PromptTokens == nil || *out[0].Metadata.TokenUsage.PromptTokens != 42 {
		t.Fatalf("prompt tokens = %+v, want 42", out[0].Metadata.TokenUsage.PromptTokens)
	}
}

func TestList_IncludeMetadataPropagatesRunError(t *testing.T) {
	t.Parallel()

	runs := &fakeRunsAccessor{
		queryRuns: []langsmithruns.Summary{{ID: "run-1", Name: "trace-a"}},
		getErr: map[string]error{
			"run-1": errors.New("run fetch failed"),
		},
	}
	svc, err := New(runs, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{
		ProjectID:       "project-123",
		IncludeMetadata: true,
	})
	if err == nil || !strings.Contains(err.Error(), "run fetch failed") {
		t.Fatalf("List() error = %v, want wrapped run fetch error", err)
	}
}

func TestList_IncludeFeedbackRequiresAccessor(t *testing.T) {
	t.Parallel()

	runs := &fakeRunsAccessor{
		queryRuns: []langsmithruns.Summary{{ID: "run-1", Name: "trace-a"}},
	}
	svc, err := New(runs, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{
		ProjectID:       "project-123",
		IncludeFeedback: true,
	})
	if err == nil || !strings.Contains(err.Error(), "feedback accessor is required") {
		t.Fatalf("List() error = %v, want feedback accessor error", err)
	}
}

func TestList_IncludeFeedbackFetchesFeedback(t *testing.T) {
	t.Parallel()

	runs := &fakeRunsAccessor{
		queryRuns: []langsmithruns.Summary{{ID: "run-1", Name: "trace-a"}},
	}
	feedback := &fakeFeedbackAccessor{
		byRun: map[string][]langsmithfeedback.Item{
			"run-1": {
				{ID: "fb-1", RunID: "run-1", Key: "correctness"},
			},
		},
	}
	svc, err := New(runs, feedback)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	out, err := svc.List(context.Background(), ListParams{
		ProjectID:       "project-123",
		IncludeFeedback: true,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(feedback.calls) != 1 || len(feedback.calls[0].RunIDs) != 1 || feedback.calls[0].RunIDs[0] != "run-1" {
		t.Fatalf("feedback calls = %+v, want run-1", feedback.calls)
	}
	if len(out[0].Feedback) != 1 || out[0].Feedback[0].ID != "fb-1" {
		t.Fatalf("feedback = %+v, want fb-1", out[0].Feedback)
	}
}

func TestList_IncludeFeedbackPropagatesError(t *testing.T) {
	t.Parallel()

	runs := &fakeRunsAccessor{
		queryRuns: []langsmithruns.Summary{{ID: "run-1", Name: "trace-a"}},
	}
	feedback := &fakeFeedbackAccessor{
		err: map[string]error{"run-1": errors.New("feedback failed")},
	}
	svc, err := New(runs, feedback)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	_, err = svc.List(context.Background(), ListParams{
		ProjectID:       "project-123",
		IncludeFeedback: true,
	})
	if err == nil || !strings.Contains(err.Error(), "feedback failed") {
		t.Fatalf("List() error = %v, want wrapped feedback error", err)
	}
}
