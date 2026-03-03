// accessor.go implements thread-domain API access via shared SDK transport.
package threads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"langsmith-sdk/go/langsmith/transport"

	"langsmith-fetch-go/internal/langsmith/statuserr"
)

// Doer is the minimal transport contract used by the threads accessor.
type Doer interface {
	Do(context.Context, transport.Request) (transport.Response, error)
}

// Accessor handles thread-oriented API calls.
type Accessor struct {
	doer Doer
}

// GetMessagesParams controls thread message fetch behavior.
type GetMessagesParams struct {
	ThreadID  string
	ProjectID string
}

// Message is a raw JSON LangSmith message payload.
//
// We keep messages as raw JSON because schemas can vary across models/tools;
// this preserves payload fidelity without untyped interface values.
type Message = json.RawMessage

// NewAccessor creates a threads accessor.
func NewAccessor(doer Doer) (*Accessor, error) {
	if doer == nil {
		return nil, fmt.Errorf("threads: doer is required")
	}
	return &Accessor{doer: doer}, nil
}

// GetMessages fetches and parses messages for a single thread.
func (a *Accessor) GetMessages(ctx context.Context, params GetMessagesParams) ([]Message, error) {
	if params.ThreadID == "" {
		return nil, fmt.Errorf("threads: thread id is required")
	}
	if params.ProjectID == "" {
		return nil, fmt.Errorf("threads: project id is required")
	}

	req := transport.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/runs/threads/%s", url.PathEscape(params.ThreadID)),
	).
		WithQuery("select", "all_messages").
		WithQuery("session_id", params.ProjectID)

	resp, err := a.doer.Do(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("threads: fetch thread: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, statuserr.Wrap("threads: fetch thread", resp.StatusCode, resp.Body)
	}

	var payload struct {
		Previews struct {
			AllMessages string `json:"all_messages"`
		} `json:"previews"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return nil, fmt.Errorf("threads: decode response: %w", err)
	}
	if payload.Previews.AllMessages == "" {
		return nil, fmt.Errorf("threads: response missing previews.all_messages")
	}

	parts := strings.Split(payload.Previews.AllMessages, "\n\n")
	messages := make([]Message, 0, len(parts))
	for i, part := range parts {
		chunk := strings.TrimSpace(part)
		if chunk == "" {
			continue
		}
		var msg json.RawMessage
		if err := json.Unmarshal([]byte(chunk), &msg); err != nil {
			return nil, fmt.Errorf("threads: decode message %d: %w", i, err)
		}
		messages = append(messages, Message(msg))
	}

	return messages, nil
}
