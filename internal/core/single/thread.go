// thread.go orchestrates single-thread fetch behavior using the threads accessor.
package single

import (
	"context"
	"fmt"

	langsmiththreads "langsmith-fetch-go/internal/langsmith/threads"
)

type threadsAccessor interface {
	GetMessages(context.Context, langsmiththreads.GetMessagesParams) ([]langsmiththreads.Message, error)
}

// ThreadService handles single thread retrieval flows.
type ThreadService struct {
	threads threadsAccessor
}

// ThreadParams controls single thread fetch behavior.
type ThreadParams struct {
	ThreadID  string
	ProjectID string
}

// ThreadMessage is a raw JSON thread message payload.
type ThreadMessage = langsmiththreads.Message

// NewThreadService creates a single-thread service.
func NewThreadService(accessor threadsAccessor) (*ThreadService, error) {
	if accessor == nil {
		return nil, fmt.Errorf("single thread: threads accessor is required")
	}
	return &ThreadService{threads: accessor}, nil
}

// GetMessages fetches messages for a single thread.
func (s *ThreadService) GetMessages(ctx context.Context, params ThreadParams) ([]ThreadMessage, error) {
	if params.ThreadID == "" {
		return nil, fmt.Errorf("single thread: thread id is required")
	}
	if params.ProjectID == "" {
		return nil, fmt.Errorf("single thread: project id is required")
	}

	messages, err := s.threads.GetMessages(ctx, langsmiththreads.GetMessagesParams{
		ThreadID:  params.ThreadID,
		ProjectID: params.ProjectID,
	})
	if err != nil {
		return nil, fmt.Errorf("single thread: fetch thread: %w", err)
	}
	return messages, nil
}
