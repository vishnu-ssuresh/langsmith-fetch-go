// service.go orchestrates multi-thread listing using runs and threads accessors.
package threads

import (
	"context"
	"fmt"

	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
	langsmiththreads "langsmith-fetch-go/internal/langsmith/threads"
)

const (
	defaultThreadListLimit = 20
	threadListOverfetch    = 10
	threadListMinQuery     = 100
)

type listRunsAccessor interface {
	QueryRootRuns(context.Context, langsmithruns.QueryRootParams) ([]langsmithruns.RootRun, error)
}

type listThreadsAccessor interface {
	GetMessages(context.Context, langsmiththreads.GetMessagesParams) ([]langsmiththreads.Message, error)
}

// Message is the thread message type returned by the threads accessor.
type Message = langsmiththreads.Message

// Lister handles bulk thread retrieval flows.
type Lister struct {
	runs    listRunsAccessor
	threads listThreadsAccessor
}

// ListParams controls bulk thread list behavior.
type ListParams struct {
	ProjectID string
	Limit     int
}

// ThreadData is the thread payload returned by bulk listing.
type ThreadData struct {
	ThreadID string    `json:"thread_id"`
	Messages []Message `json:"messages"`
}

// NewLister creates a bulk thread lister.
func NewLister(runs listRunsAccessor, threads listThreadsAccessor) (*Lister, error) {
	if runs == nil {
		return nil, fmt.Errorf("threads: runs accessor is required")
	}
	if threads == nil {
		return nil, fmt.Errorf("threads: threads accessor is required")
	}
	return &Lister{runs: runs, threads: threads}, nil
}

// List fetches recent unique threads for a project.
//
// The run query overfetches to increase the chance of getting enough unique
// thread IDs when multiple root runs belong to the same thread.
func (l *Lister) List(ctx context.Context, params ListParams) ([]ThreadData, error) {
	if params.ProjectID == "" {
		return nil, fmt.Errorf("threads: project id is required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultThreadListLimit
	}

	queryLimit := limit * threadListOverfetch
	if queryLimit < threadListMinQuery {
		queryLimit = threadListMinQuery
	}

	runs, err := l.runs.QueryRootRuns(ctx, langsmithruns.QueryRootParams{
		ProjectID: params.ProjectID,
		Limit:     queryLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("threads: query root runs: %w", err)
	}

	threadOrder := uniqueThreadIDs(runs, limit)
	out := make([]ThreadData, 0, len(threadOrder))
	for _, threadID := range threadOrder {
		messages, err := l.threads.GetMessages(ctx, langsmiththreads.GetMessagesParams{
			ThreadID:  threadID,
			ProjectID: params.ProjectID,
		})
		if err != nil {
			return nil, fmt.Errorf("threads: fetch thread %q: %w", threadID, err)
		}
		out = append(out, ThreadData{
			ThreadID: threadID,
			Messages: messages,
		})
	}

	return out, nil
}

func uniqueThreadIDs(runs []langsmithruns.RootRun, limit int) []string {
	if limit <= 0 {
		return nil
	}

	out := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, run := range runs {
		threadID := run.ThreadID
		if threadID == "" {
			continue
		}
		if _, ok := seen[threadID]; ok {
			continue
		}
		seen[threadID] = struct{}{}
		out = append(out, threadID)
		if len(out) >= limit {
			break
		}
	}
	return out
}
