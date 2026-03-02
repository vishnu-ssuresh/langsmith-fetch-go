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
	if params.TraceID == "" {
		return nil, fmt.Errorf("single trace: trace id is required")
	}

	run, err := s.runs.GetRun(ctx, langsmithruns.GetRunParams{
		RunID:           params.TraceID,
		IncludeMessages: true,
	})
	if err != nil {
		return nil, fmt.Errorf("single trace: fetch trace: %w", err)
	}
	if len(run.Messages) > 0 {
		return run.Messages, nil
	}
	if len(run.Outputs.Messages) > 0 {
		return run.Outputs.Messages, nil
	}
	return []Message{}, nil
}
