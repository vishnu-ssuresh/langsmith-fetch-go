package projects

import (
	"context"
	"fmt"
	"strings"

	"github.com/langchain-ai/langsmith-go"
)

// Accessor handles project-oriented API calls via the official SDK.
type Accessor struct {
	client *langsmith.Client
}

// NewAccessor creates a projects accessor backed by the official SDK.
func NewAccessor(client *langsmith.Client) (*Accessor, error) {
	if client == nil {
		return nil, fmt.Errorf("projects: client is required")
	}
	return &Accessor{client: client}, nil
}

// ResolveProjectUUID resolves a project UUID by project name using the SDK's Sessions.List.
func (a *Accessor) ResolveProjectUUID(ctx context.Context, projectName string) (string, error) {
	projectName = strings.TrimSpace(projectName)
	if projectName == "" {
		return "", fmt.Errorf("projects: project name is required")
	}

	page, err := a.client.Sessions.List(ctx, langsmith.SessionListParams{
		Name:         langsmith.F(projectName),
		Limit:        langsmith.F(int64(1)),
		IncludeStats: langsmith.F(false),
	})
	if err != nil {
		return "", fmt.Errorf("projects: lookup project: %w", err)
	}

	if len(page.Items) == 0 {
		return "", fmt.Errorf("projects: project %q not found", projectName)
	}
	if page.Items[0].ID == "" {
		return "", fmt.Errorf("projects: project %q not found", projectName)
	}
	return page.Items[0].ID, nil
}
