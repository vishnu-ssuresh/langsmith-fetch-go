// service.go orchestrates trace listing using the runs domain accessor.
package traces

import (
	"context"
	"fmt"

	langsmithruns "langsmith-fetch-go/internal/langsmith/runs"
)

type runsAccessor interface {
	QueryRoot(context.Context, langsmithruns.QueryRootParams) ([]langsmithruns.Summary, error)
}

// Service handles trace-oriented API calls.
type Service struct {
	runs runsAccessor
}

// ListParams controls trace listing behavior.
type ListParams struct {
	ProjectID string
	Limit     int
}

// Summary is the minimal trace information returned by List.
type Summary = langsmithruns.Summary

// New creates a traces service.
func New(accessor runsAccessor) (*Service, error) {
	if accessor == nil {
		return nil, fmt.Errorf("traces: runs accessor is required")
	}
	return &Service{runs: accessor}, nil
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

	runs, err := s.runs.QueryRoot(ctx, langsmithruns.QueryRootParams{
		ProjectID: params.ProjectID,
		Limit:     limit,
	})
	if err != nil {
		return nil, fmt.Errorf("traces: query traces: %w", err)
	}
	return runs, nil
}
