// accessor.go implements run-domain API access via shared SDK transport.
package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"langsmith-sdk/go/langsmith/transport"
)

// Doer is the minimal transport contract used by the runs accessor.
type Doer interface {
	Do(context.Context, transport.Request) (transport.Response, error)
}

// Accessor handles runs-oriented API calls.
type Accessor struct {
	doer Doer
}

// QueryRootParams controls root-run query behavior.
type QueryRootParams struct {
	ProjectID string
	Limit     int
}

// Summary is the minimal run/trace information returned by QueryRoot.
type Summary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StartTime string `json:"start_time"`
}

type queryRunsRequest struct {
	Session []string `json:"session"`
	IsRoot  bool     `json:"is_root"`
	Limit   int      `json:"limit"`
}

// NewAccessor creates a runs accessor.
func NewAccessor(doer Doer) (*Accessor, error) {
	if doer == nil {
		return nil, fmt.Errorf("runs: doer is required")
	}
	return &Accessor{doer: doer}, nil
}

// QueryRoot fetches recent root runs for a project.
func (a *Accessor) QueryRoot(ctx context.Context, params QueryRootParams) ([]Summary, error) {
	if params.ProjectID == "" {
		return nil, fmt.Errorf("runs: project id is required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	body := queryRunsRequest{
		Session: []string{params.ProjectID},
		IsRoot:  true,
		Limit:   limit,
	}
	bodyBytes, err := transport.EncodeJSONBody(body)
	if err != nil {
		return nil, fmt.Errorf("runs: encode request body: %w", err)
	}

	req := transport.NewRequest(http.MethodPost, "/runs/query").WithBody(bodyBytes)
	resp, err := a.doer.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("runs: query runs: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf(
			"runs: query runs returned status %d: %s",
			resp.StatusCode,
			string(resp.Body),
		)
	}

	var payload struct {
		Runs []Summary `json:"runs"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("runs: decode response: %w", err)
	}
	return payload.Runs, nil
}
