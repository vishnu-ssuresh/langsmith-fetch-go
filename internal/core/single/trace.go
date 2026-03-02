// trace.go orchestrates single-trace fetch behavior using the runs accessor.
package single

import (
	"context"
	"fmt"

	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
)

type runsAccessor interface {
	GetRun(context.Context, langsmithruns.GetRunParams) (langsmithruns.Run, error)
}

// TraceService handles single trace retrieval flows.
type TraceService struct {
	runs runsAccessor
}

// TraceParams controls single trace fetch behavior.
type TraceParams struct {
	TraceID string
}

// Message is a raw JSON LangSmith message payload.
type Message = langsmithruns.Message

// Run is the LangSmith run payload used for optional metadata extraction.
type Run = langsmithruns.Run

// NewTraceService creates a single-trace service.
func NewTraceService(accessor runsAccessor) (*TraceService, error) {
	if accessor == nil {
		return nil, fmt.Errorf("single trace: runs accessor is required")
	}
	return &TraceService{runs: accessor}, nil
}

// GetMessages fetches messages for a single trace.
//
// Extraction order is:
// 1. run.messages
// 2. run.outputs.messages
func (s *TraceService) GetMessages(ctx context.Context, params TraceParams) ([]Message, error) {
	run, err := s.GetRun(ctx, params)
	if err != nil {
		return nil, err
	}
	if len(run.Messages) > 0 {
		return run.Messages, nil
	}
	if len(run.Outputs.Messages) > 0 {
		return run.Outputs.Messages, nil
	}
	return []Message{}, nil
}

// GetRun fetches the raw run payload for a trace.
func (s *TraceService) GetRun(ctx context.Context, params TraceParams) (Run, error) {
	if params.TraceID == "" {
		return Run{}, fmt.Errorf("single trace: trace id is required")
	}

	run, err := s.runs.GetRun(ctx, langsmithruns.GetRunParams{
		RunID:           params.TraceID,
		IncludeMessages: true,
	})
	if err != nil {
		return Run{}, fmt.Errorf("single trace: fetch trace: %w", err)
	}
	return run, nil
}
