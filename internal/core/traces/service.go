package traces

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"langsmith-sdk/go/langsmith/transport"
)

// Doer is the minimal transport contract used by the traces service.
type Doer interface {
	Do(context.Context, transport.Request) (transport.Response, error)
}

// Service handles trace-oriented API calls.
type Service struct {
	doer Doer
}

// ListParams controls trace listing behavior.
type ListParams struct {
	ProjectID string
	Limit     int
}

// Summary is the minimal trace information returned by List.
type Summary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StartTime string `json:"start_time"`
}

// New creates a traces service.
func New(doer Doer) (*Service, error) {
	if doer == nil {
		return nil, fmt.Errorf("traces: doer is required")
	}
	return &Service{doer: doer}, nil
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

	body := map[string]any{
		"session": []string{params.ProjectID},
		"is_root": true,
		"limit":   limit,
	}

	resp, err := s.doer.Do(ctx, transport.Request{
		Method: http.MethodPost,
		Path:   "/runs/query",
		Body:   body,
	})
	if err != nil {
		return nil, fmt.Errorf("traces: query traces: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf(
			"traces: query traces returned status %d: %s",
			resp.StatusCode,
			string(resp.Body),
		)
	}

	var payload struct {
		Runs []Summary `json:"runs"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("traces: decode response: %w", err)
	}
	return payload.Runs, nil
}
