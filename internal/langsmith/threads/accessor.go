package threads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/langchain-ai/langsmith-go"
)

// Accessor handles thread-oriented API calls via the official SDK.
type Accessor struct {
	client *langsmith.Client
}

// GetMessagesParams controls thread message fetch behavior.
type GetMessagesParams struct {
	ThreadID  string
	ProjectID string
}

// Message is a raw JSON LangSmith message payload.
type Message = json.RawMessage

// NewAccessor creates a threads accessor backed by the official SDK.
func NewAccessor(client *langsmith.Client) (*Accessor, error) {
	if client == nil {
		return nil, fmt.Errorf("threads: client is required")
	}
	return &Accessor{client: client}, nil
}

// threadResponse matches the LangSmith thread API response shape.
type threadResponse struct {
	Previews struct {
		AllMessages *string `json:"all_messages"`
	} `json:"previews"`
}

// GetMessages fetches and parses messages for a single thread.
// Uses the SDK's generic Get for the undocumented /runs/threads endpoint.
func (a *Accessor) GetMessages(ctx context.Context, params GetMessagesParams) ([]Message, error) {
	if params.ThreadID == "" {
		return nil, fmt.Errorf("threads: thread id is required")
	}
	if params.ProjectID == "" {
		return nil, fmt.Errorf("threads: project id is required")
	}

	path := fmt.Sprintf(
		"api/v1/runs/threads/%s?select=all_messages&session_id=%s",
		url.PathEscape(params.ThreadID),
		url.QueryEscape(params.ProjectID),
	)

	var payload threadResponse
	err := a.client.Get(ctx, path, nil, &payload)
	if err != nil {
		return nil, fmt.Errorf("threads: fetch thread: %w", err)
	}
	if payload.Previews.AllMessages == nil {
		return nil, fmt.Errorf("threads: response missing previews.all_messages")
	}
	if strings.TrimSpace(*payload.Previews.AllMessages) == "" {
		return []Message{}, nil
	}

	parts := strings.Split(*payload.Previews.AllMessages, "\n\n")
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
