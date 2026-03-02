// service.go orchestrates multi-thread listing using runs and threads accessors.
package threads

import (
	"context"
	"fmt"
	"sync"

	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
	langsmiththreads "langsmith-fetch-go/internal/langsmith/threads"
)

const (
	defaultThreadListLimit = 20
	defaultMaxConcurrent   = 5
	maxAllowedConcurrent   = 100
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
	ProjectID     string
	Limit         int
	StartTime     string
	MaxConcurrent int
	ShowProgress  bool
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
		StartTime: params.StartTime,
	})
	if err != nil {
		return nil, fmt.Errorf("threads: query root runs: %w", err)
	}

	threadOrder := uniqueThreadIDs(runs, limit)
	out := make([]ThreadData, len(threadOrder))
	if len(threadOrder) == 0 {
		return out, nil
	}

	maxConcurrent := normalizeMaxConcurrent(params.MaxConcurrent)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sem := make(chan struct{}, maxConcurrent)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	for index, threadID := range threadOrder {
		index := index
		threadID := threadID
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			messages, err := l.threads.GetMessages(ctx, langsmiththreads.GetMessagesParams{
				ThreadID:  threadID,
				ProjectID: params.ProjectID,
			})
			if err != nil {
				select {
				case errCh <- fmt.Errorf("threads: fetch thread %q: %w", threadID, err):
				default:
				}
				cancel()
				return
			}

			out[index] = ThreadData{
				ThreadID: threadID,
				Messages: messages,
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

func normalizeMaxConcurrent(value int) int {
	if value <= 0 {
		return defaultMaxConcurrent
	}
	if value > maxAllowedConcurrent {
		return maxAllowedConcurrent
	}
	return value
}
