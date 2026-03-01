package app

import (
	langsmith "langsmith-sdk/go/langsmith"

	"langsmith-fetch-go/internal/config"
	coreclient "langsmith-fetch-go/internal/core/client"
)

// NewClientFromEnv builds the shared LangSmith client using environment config.
func NewClientFromEnv() (*langsmith.Client, error) {
	cfg := config.LoadFromEnv()
	return coreclient.New(coreclient.Config{
		APIKey:      cfg.APIKey,
		WorkspaceID: cfg.WorkspaceID,
		Endpoint:    cfg.Endpoint,
	})
}
