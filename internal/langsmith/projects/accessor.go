// accessor.go implements project-domain API access via shared SDK transport.
package projects

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"langsmith-sdk/go/langsmith/transport"
)

// Doer is the minimal transport contract used by the projects accessor.
type Doer interface {
	Do(context.Context, transport.Request) (transport.Response, error)
}

// Accessor handles project-oriented API calls.
type Accessor struct {
	doer Doer
}

type session struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// NewAccessor creates a projects accessor.
func NewAccessor(doer Doer) (*Accessor, error) {
	if doer == nil {
		return nil, fmt.Errorf("projects: doer is required")
	}
	return &Accessor{doer: doer}, nil
}

// ResolveProjectUUID resolves a project UUID by project name.
//
// This follows the LangSmith project-read contract:
// GET /sessions?name=<project_name>&limit=1&include_stats=false
func (a *Accessor) ResolveProjectUUID(ctx context.Context, projectName string) (string, error) {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return "", fmt.Errorf("projects: project name is required")
	}

	req := transport.NewRequest(http.MethodGet, "/sessions").
		WithQuery("name", projectName).
		WithQuery("limit", "1").
		WithQuery("include_stats", "false")

	resp, err := a.doer.Do(ctx, req)
	if err != nil {
		return "", fmt.Errorf("projects: lookup project: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf(
			"projects: lookup project returned status %d: %s",
			resp.StatusCode,
			string(resp.Body),
		)
	}

	projectID, parseErr := parseProjectID(resp.Body)
	if parseErr != nil {
		return "", fmt.Errorf("projects: decode response: %w", parseErr)
	}
	if projectID == "" {
		return "", fmt.Errorf("projects: project %q not found", projectName)
	}
	return projectID, nil
}

func parseProjectID(body []byte) (string, error) {
	var sessions []session
	if err := json.Unmarshal(body, &sessions); err == nil {
		for _, item := range sessions {
			if item.ID != "" {
				return item.ID, nil
			}
		}
		return "", nil
	}

	var payload struct {
		ID       string    `json:"id"`
		Sessions []session `json:"sessions"`
		Results  []session `json:"results"`
		Items    []session `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	if payload.ID != "" {
		return payload.ID, nil
	}
	for _, group := range [][]session{payload.Sessions, payload.Results, payload.Items} {
		for _, item := range group {
			if item.ID != "" {
				return item.ID, nil
			}
		}
	}
	return "", nil
}
