package feedback

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/langchain-ai/langsmith-go"
	"github.com/langchain-ai/langsmith-go/shared"
)

// Accessor handles feedback-oriented API calls via the official SDK.
type Accessor struct {
	client *langsmith.Client
}

// ListParams controls feedback list behavior.
type ListParams struct {
	RunIDs []string
	Limit  int
	Keys   []string
	Source []string
}

// Item is the feedback shape consumed by fetch-go.
type Item struct {
	ID      string          `json:"id"`
	RunID   string          `json:"run_id"`
	Key     string          `json:"key"`
	Score   json.RawMessage `json:"score"`
	Value   json.RawMessage `json:"value"`
	Comment string          `json:"comment"`
}

// NewAccessor creates a feedback accessor backed by the official SDK.
func NewAccessor(client *langsmith.Client) (*Accessor, error) {
	if client == nil {
		return nil, fmt.Errorf("feedback: client is required")
	}
	return &Accessor{client: client}, nil
}

// ListByRuns lists feedback records for one or more run IDs.
func (a *Accessor) ListByRuns(ctx context.Context, params ListParams) ([]Item, error) {
	if len(params.RunIDs) == 0 {
		return nil, fmt.Errorf("feedback: at least one run id is required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 100 {
		limit = 100
	}

	query := langsmith.FeedbackListParams{
		Run:   langsmith.F[langsmith.FeedbackListParamsRunUnion](langsmith.FeedbackListParamsRunArray(params.RunIDs)),
		Limit: langsmith.F(int64(limit)),
	}
	if len(params.Keys) > 0 {
		query.Key = langsmith.F(params.Keys)
	}
	if len(params.Source) > 0 {
		sources := make([]langsmith.SourceType, len(params.Source))
		for i, s := range params.Source {
			sources[i] = langsmith.SourceType(s)
		}
		query.Source = langsmith.F(sources)
	}

	page, err := a.client.Feedback.List(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("feedback: list feedback: %w", err)
	}

	items := make([]Item, 0)
	for _, fb := range page.Items {
		item := Item{
			ID:      fb.ID,
			RunID:   fb.RunID,
			Key:     fb.Key,
			Comment: fb.Comment,
		}
		if fb.Score != nil {
			item.Score = marshalUnion(fb.Score)
		}
		if fb.Value != nil {
			item.Value = marshalUnion(fb.Value)
		}
		items = append(items, item)
	}
	return items, nil
}

func marshalUnion(v interface{}) json.RawMessage {
	switch val := v.(type) {
	case shared.UnionFloat:
		data, _ := json.Marshal(float64(val))
		return data
	case shared.UnionBool:
		data, _ := json.Marshal(bool(val))
		return data
	case shared.UnionString:
		data, _ := json.Marshal(string(val))
		return data
	default:
		data, _ := json.Marshal(v)
		return data
	}
}
