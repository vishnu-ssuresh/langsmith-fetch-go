// accessor.go implements feedback-domain API access via shared SDK transport.
package feedback

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"langsmith-sdk/go/langsmith/transport"
)

const (
	defaultLimit = 100
	maxLimit     = 100
)

// Doer is the minimal transport contract used by the feedback accessor.
type Doer interface {
	Do(context.Context, transport.Request) (transport.Response, error)
}

// Accessor handles feedback-oriented API calls.
type Accessor struct {
	doer Doer
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

// NewAccessor creates a feedback accessor.
func NewAccessor(doer Doer) (*Accessor, error) {
	if doer == nil {
		return nil, fmt.Errorf("feedback: doer is required")
	}
	return &Accessor{doer: doer}, nil
}

// ListByRuns lists feedback records for one or more run IDs.
func (a *Accessor) ListByRuns(ctx context.Context, params ListParams) ([]Item, error) {
	runIDs := sanitizeStrings(params.RunIDs)
	if len(runIDs) == 0 {
		return nil, fmt.Errorf("feedback: at least one run id is required")
	}

	limit := params.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	req := transport.NewRequest(http.MethodGet, "/feedback").
		WithQuery("limit", strconv.Itoa(limit)).
		WithQuery("run", runIDs...)

	keys := sanitizeStrings(params.Keys)
	if len(keys) > 0 {
		req = req.WithQuery("key", keys...)
	}
	source := sanitizeStrings(params.Source)
	if len(source) > 0 {
		req = req.WithQuery("source", source...)
	}

	resp, err := a.doer.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("feedback: list feedback: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf(
			"feedback: list feedback returned status %d: %s",
			resp.StatusCode,
			string(resp.Body),
		)
	}

	items, err := decodeItems(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("feedback: decode response: %w", err)
	}
	return items, nil
}

func decodeItems(body []byte) ([]Item, error) {
	var arr []Item
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}

	var wrapped struct {
		Items    []Item `json:"items"`
		Results  []Item `json:"results"`
		Feedback []Item `json:"feedback"`
	}
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, err
	}

	for _, group := range [][]Item{wrapped.Items, wrapped.Results, wrapped.Feedback} {
		if len(group) > 0 {
			return group, nil
		}
	}
	return []Item{}, nil
}

func sanitizeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}
