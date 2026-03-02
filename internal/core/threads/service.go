// service.go orchestrates thread fetch flows using the threads domain accessor.
package threads

import (
	"context"
	"fmt"

	langsmiththreads "langsmith-fetch-go/internal/langsmith/threads"
)

type threadsAccessor interface {
	GetMessages(context.Context, langsmiththreads.GetMessagesParams) ([]langsmiththreads.Message, error)
}

// Service handles thread-oriented API calls.
type Service struct {
	threads threadsAccessor
}

// GetParams controls thread fetch behavior.
type GetParams struct {
	ThreadID  string
	ProjectID string
}

// Message is the thread message type returned by the threads accessor.
type Message = langsmiththreads.Message

// New creates a threads service.
func New(accessor threadsAccessor) (*Service, error) {
	if accessor == nil {
		return nil, fmt.Errorf("threads: threads accessor is required")
	}
	return &Service{threads: accessor}, nil
}

// GetMessages fetches and parses thread messages.
func (s *Service) GetMessages(ctx context.Context, params GetParams) ([]Message, error) {
	if params.ThreadID == "" {
		return nil, fmt.Errorf("threads: thread id is required")
	}
	if params.ProjectID == "" {
		return nil, fmt.Errorf("threads: project id is required")
	}

	messages, err := s.threads.GetMessages(ctx, langsmiththreads.GetMessagesParams{
		ThreadID:  params.ThreadID,
		ProjectID: params.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("threads: fetch thread: %w", err)
	}
	return messages, nil
}
