// accessor.go implements run-domain API access via shared SDK transport.
package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

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
	StartTime string
}

// RootRun is the root-run shape needed for thread list construction.
type RootRun struct {
	ID        string
	Name      string
	StartTime string
	ThreadID  string
}

// GetRunParams controls single-run fetch behavior.
type GetRunParams struct {
	RunID           string
	IncludeMessages bool
}

// Summary is the minimal run/trace information returned by QueryRoot.
type Summary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	StartTime string `json:"start_time"`
}

// Message is a raw JSON LangSmith message payload.
type Message = json.RawMessage

// Run contains single-run fields used by fetch-go.
type Run struct {
	ID               string          `json:"id"`
	Status           string          `json:"status"`
	StartTime        string          `json:"start_time"`
	EndTime          string          `json:"end_time"`
	PromptTokens     *int            `json:"prompt_tokens"`
	CompletionTokens *int            `json:"completion_tokens"`
	TotalTokens      *int            `json:"total_tokens"`
	PromptCost       *float64        `json:"prompt_cost"`
	CompletionCost   *float64        `json:"completion_cost"`
	TotalCost        *float64        `json:"total_cost"`
	FirstTokenTime   string          `json:"first_token_time"`
	FeedbackStats    json.RawMessage `json:"feedback_stats"`
	Extra            Extra           `json:"extra"`
	Messages         []Message       `json:"messages"`
	Outputs          Outputs         `json:"outputs"`
}

// Outputs is the run output envelope.
type Outputs struct {
	Messages []Message `json:"messages"`
}

// Extra is the run extra envelope.
type Extra struct {
	Metadata json.RawMessage `json:"metadata"`
}

type queryRunsRequest struct {
	Session   []string `json:"session"`
	IsRoot    bool     `json:"is_root"`
	Limit     int      `json:"limit"`
	StartTime string   `json:"start_time,omitempty"`
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
	runs, err := a.QueryRootRuns(ctx, params)
	if err != nil {
		return nil, err
	}

	out := make([]Summary, 0, len(runs))
	for _, run := range runs {
		out = append(out, Summary{
			ID:        run.ID,
			Name:      run.Name,
			StartTime: run.StartTime,
		})
	}
	return out, nil
}

// QueryRootRuns fetches recent root runs with thread metadata.
func (a *Accessor) QueryRootRuns(ctx context.Context, params QueryRootParams) ([]RootRun, error) {
	if params.ProjectID == "" {
		return nil, fmt.Errorf("runs: project id is required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}

	body := queryRunsRequest{
		Session:   []string{params.ProjectID},
		IsRoot:    true,
		Limit:     limit,
		StartTime: params.StartTime,
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
		Runs []struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			StartTime string `json:"start_time"`
			Extra     struct {
				Metadata struct {
					ThreadID string `json:"thread_id"`
				} `json:"metadata"`
			} `json:"extra"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("runs: decode response: %w", err)
	}

	runs := make([]RootRun, 0, len(payload.Runs))
	for _, item := range payload.Runs {
		runs = append(runs, RootRun{
			ID:        item.ID,
			Name:      item.Name,
			StartTime: item.StartTime,
			ThreadID:  item.Extra.Metadata.ThreadID,
		})
	}
	return runs, nil
}

// GetRun fetches a single run by ID.
func (a *Accessor) GetRun(ctx context.Context, params GetRunParams) (Run, error) {
	if params.RunID == "" {
		return Run{}, fmt.Errorf("runs: run id is required")
	}

	req := transport.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/runs/%s", url.PathEscape(params.RunID)),
	)
	if params.IncludeMessages {
		req = req.WithQuery("include_messages", "true")
	}

	resp, err := a.doer.Do(ctx, req)
	if err != nil {
		return Run{}, fmt.Errorf("runs: get run: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return Run{}, fmt.Errorf(
			"runs: get run returned status %d: %s",
			resp.StatusCode,
			string(resp.Body),
		)
	}

	var run Run
	if err := json.Unmarshal(resp.Body, &run); err != nil {
		return Run{}, fmt.Errorf("runs: decode response: %w", err)
	}
	return run, nil
}
