package threads

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"langsmith-sdk/go/langsmith/transport"
)

// Doer is the minimal transport contract used by the threads service.
type Doer interface {
	Do(context.Context, transport.Request) (transport.Response, error)
}

// Service handles thread-oriented API calls.
type Service struct {
	doer Doer
}

// GetParams controls thread fetch behavior.
type GetParams struct {
	ThreadID  string
	ProjectID string
}

// Message is a raw JSON LangSmith message payload.
//
// We keep messages as raw JSON because schemas can vary across models/tools;
// this preserves payload fidelity without untyped interface values.
type Message = json.RawMessage

// New creates a threads service.
func New(doer Doer) (*Service, error) {
	if doer == nil {
		return nil, fmt.Errorf("threads: doer is required")
	}
	return &Service{doer: doer}, nil
}

// GetMessages fetches and parses thread messages.
func (s *Service) GetMessages(ctx context.Context, params GetParams) ([]Message, error) {
	if params.ThreadID == "" {
		return nil, fmt.Errorf("threads: thread id is required")
	}
	if params.ProjectID == "" {
		return nil, fmt.Errorf("threads: project id is required")
	}

	resp, err := s.doer.Do(ctx, transport.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("/runs/threads/%s", url.PathEscape(params.ThreadID)),
		Query: url.Values{
			"select":     []string{"all_messages"},
			"session_id": []string{params.ProjectID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("threads: fetch thread: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf(
			"threads: fetch thread returned status %d: %s",
			resp.StatusCode,
			string(resp.Body),
		)
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
