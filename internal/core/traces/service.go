// service.go orchestrates trace listing using the runs domain accessor.
package traces

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	langsmithfeedback "langsmith-fetch-go/internal/langsmith/feedback"
	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
)

const (
	defaultMaxConcurrent = 5
	maxAllowedConcurrent = 100
)

type runsAccessor interface {
	QueryRoot(context.Context, langsmithruns.QueryRootParams) ([]langsmithruns.Summary, error)
	GetRun(context.Context, langsmithruns.GetRunParams) (langsmithruns.Run, error)
}

type feedbackAccessor interface {
	ListByRuns(context.Context, langsmithfeedback.ListParams) ([]langsmithfeedback.Item, error)
}

// Service handles trace-oriented API calls.
type Service struct {
	runs     runsAccessor
	feedback feedbackAccessor
}

// ListParams controls trace listing behavior.
type ListParams struct {
	ProjectID       string
	Limit           int
	StartTime       string
	IncludeMetadata bool
	IncludeFeedback bool
	MaxConcurrent   int
	ShowProgress    bool
	Progress        func(completed int, total int)
}

// TraceMetadata is additional metadata for a trace.
type TraceMetadata struct {
	Status        string          `json:"status,omitempty"`
	StartTime     string          `json:"start_time,omitempty"`
	EndTime       string          `json:"end_time,omitempty"`
	DurationMS    *int64          `json:"duration_ms,omitempty"`
	CustomMeta    json.RawMessage `json:"custom_metadata,omitempty"`
	TokenUsage    TokenUsage      `json:"token_usage"`
	Costs         Costs           `json:"costs"`
	FirstTokenAt  string          `json:"first_token_time,omitempty"`
	FeedbackStats json.RawMessage `json:"feedback_stats,omitempty"`
}

// TokenUsage contains token accounting metadata.
type TokenUsage struct {
	PromptTokens     *int `json:"prompt_tokens,omitempty"`
	CompletionTokens *int `json:"completion_tokens,omitempty"`
	TotalTokens      *int `json:"total_tokens,omitempty"`
}

// Costs contains run cost metadata.
type Costs struct {
	PromptCost     *float64 `json:"prompt_cost,omitempty"`
	CompletionCost *float64 `json:"completion_cost,omitempty"`
	TotalCost      *float64 `json:"total_cost,omitempty"`
}

// TraceData is the trace shape returned by List.
type TraceData struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	StartTime string                   `json:"start_time"`
	Metadata  *TraceMetadata           `json:"metadata,omitempty"`
	Feedback  []langsmithfeedback.Item `json:"feedback,omitempty"`
}

// Summary is kept as an alias for command/output compatibility.
type Summary = TraceData

// FeedbackItem is re-exported for output/test package boundaries.
type FeedbackItem = langsmithfeedback.Item

// New creates a traces service.
func New(runs runsAccessor, feedback feedbackAccessor) (*Service, error) {
	if runs == nil {
		return nil, fmt.Errorf("traces: runs accessor is required")
	}
	return &Service{
		runs:     runs,
		feedback: feedback,
	}, nil
}

// List fetches recent root traces for a project.
func (s *Service) List(ctx context.Context, params ListParams) ([]Summary, error) {
	if params.ProjectID == "" {
		return nil, fmt.Errorf("traces: project id is required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	rootRuns, err := s.runs.QueryRoot(ctx, langsmithruns.QueryRootParams{
		ProjectID: params.ProjectID,
		Limit:     limit,
		StartTime: params.StartTime,
	})
	if err != nil {
		return nil, fmt.Errorf("traces: query traces: %w", err)
	}

	if params.IncludeFeedback && s.feedback == nil {
		return nil, fmt.Errorf("traces: feedback accessor is required when include feedback is enabled")
	}

	out := make([]Summary, len(rootRuns))
	for i, run := range rootRuns {
		out[i] = Summary{
			ID:        run.ID,
			Name:      run.Name,
			StartTime: run.StartTime,
		}
	}
	if !params.IncludeMetadata && !params.IncludeFeedback {
		return out, nil
	}

	reportProgress := makeProgressReporter(
		params.ShowProgress,
		params.Progress,
		len(rootRuns),
	)
	reportProgress(0)
	var completed atomic.Int64

	maxConcurrent := normalizeMaxConcurrent(params.MaxConcurrent)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, maxConcurrent)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	for i, run := range rootRuns {
		i := i
		run := run
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				reportProgress(int(completed.Add(1)))
			}()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			if params.IncludeMetadata {
				fullRun, err := s.runs.GetRun(ctx, langsmithruns.GetRunParams{
					RunID: run.ID,
				})
				if err != nil {
					select {
					case errCh <- fmt.Errorf("traces: fetch trace metadata for %q: %w", run.ID, err):
					default:
					}
					cancel()
					return
				}
				metadata := extractTraceMetadata(fullRun)
				out[i].Metadata = &metadata
			}

			if params.IncludeFeedback {
				feedbackItems, err := s.feedback.ListByRuns(ctx, langsmithfeedback.ListParams{
					RunIDs: []string{run.ID},
				})
				if err != nil {
					select {
					case errCh <- fmt.Errorf("traces: fetch trace feedback for %q: %w", run.ID, err):
					default:
					}
					cancel()
					return
				}
				out[i].Feedback = feedbackItems
			}
		}()
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return nil, err
	default:
	}
	return out, nil
}

func extractTraceMetadata(run langsmithruns.Run) TraceMetadata {
	return TraceMetadata{
		Status:    run.Status,
		StartTime: run.StartTime,
		EndTime:   run.EndTime,
		DurationMS: parseDurationMilliseconds(
			run.StartTime,
			run.EndTime,
		),
		CustomMeta: run.Extra.Metadata,
		TokenUsage: TokenUsage{
			PromptTokens:     run.PromptTokens,
			CompletionTokens: run.CompletionTokens,
			TotalTokens:      run.TotalTokens,
		},
		Costs: Costs{
			PromptCost:     run.PromptCost,
			CompletionCost: run.CompletionCost,
			TotalCost:      run.TotalCost,
		},
		FirstTokenAt:  run.FirstTokenTime,
		FeedbackStats: run.FeedbackStats,
	}
}

func parseDurationMilliseconds(startTime, endTime string) *int64 {
	if startTime == "" || endTime == "" {
		return nil
	}

	start, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil
	}
	end, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil
	}

	durationMS := end.Sub(start).Milliseconds()
	return &durationMS
}

func normalizeMaxConcurrent(value int) int {
	if value <= 0 {
		return defaultMaxConcurrent
	}
	if value > maxAllowedConcurrent {
		return maxAllowedConcurrent
	}
	return value
}

func makeProgressReporter(
	enabled bool,
	callback func(completed int, total int),
	total int,
) func(completed int) {
	if !enabled || callback == nil || total <= 0 {
		return func(int) {}
	}
	var mu sync.Mutex
	return func(completed int) {
		mu.Lock()
		defer mu.Unlock()
		callback(completed, total)
	}
}
