package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/langchain-ai/langsmith-go"
)

// Accessor handles runs-oriented API calls via the official SDK.
type Accessor struct {
	client *langsmith.Client
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

// NewAccessor creates a runs accessor backed by the official SDK.
func NewAccessor(client *langsmith.Client) (*Accessor, error) {
	if client == nil {
		return nil, fmt.Errorf("runs: client is required")
	}
	return &Accessor{client: client}, nil
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

	queryParams := langsmith.RunQueryParams{
		Session: langsmith.F([]string{params.ProjectID}),
		IsRoot:  langsmith.F(true),
		Limit:   langsmith.F(int64(limit)),
	}
	if params.StartTime != "" {
		t, err := time.Parse(time.RFC3339, params.StartTime)
		if err == nil {
			queryParams.StartTime = langsmith.F(t)
		}
	}

	resp, err := a.client.Runs.Query(ctx, queryParams)
	if err != nil {
		return nil, fmt.Errorf("runs: query runs: %w", err)
	}

	runs := make([]RootRun, 0, len(resp.Runs))
	for _, item := range resp.Runs {
		threadID := item.ThreadID
		startTime := ""
		if !item.StartTime.IsZero() {
			startTime = item.StartTime.Format(time.RFC3339)
		}
		runs = append(runs, RootRun{
			ID:        item.ID,
			Name:      item.Name,
			StartTime: startTime,
			ThreadID:  threadID,
		})
	}
	return runs, nil
}

// GetRun fetches a single run by ID via the SDK's generic endpoint support.
func (a *Accessor) GetRun(ctx context.Context, params GetRunParams) (Run, error) {
	if params.RunID == "" {
		return Run{}, fmt.Errorf("runs: run id is required")
	}

	path := fmt.Sprintf("api/v1/runs/%s", url.PathEscape(params.RunID))
	if params.IncludeMessages {
		path = path + "?include_messages=true"
	}

	var raw Run
	err := a.client.Get(ctx, path, nil, &raw)
	if err != nil {
		return Run{}, fmt.Errorf("runs: get run: %w", err)
	}
	return raw, nil
}
