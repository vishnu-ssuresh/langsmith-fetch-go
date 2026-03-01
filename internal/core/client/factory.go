package client

import (
	"time"

	langsmith "langsmith-sdk/go/langsmith"
)

// DefaultUserAgent identifies requests from langsmith-fetch-go.
const DefaultUserAgent = "langsmith-fetch-go"

// Config defines the inputs needed to construct the shared SDK client.
type Config struct {
	APIKey      string
	WorkspaceID string
	Endpoint    string
	UserAgent   string
	Timeout     time.Duration
	RetryMax    int
}

// New creates a LangSmith SDK client from fetch-go config.
func New(cfg Config) (*langsmith.Client, error) {
	return langsmith.NewClient(buildOptions(cfg))
}

func buildOptions(cfg Config) langsmith.ClientOptions {
	opts := langsmith.ClientOptions{
		APIKey:      cfg.APIKey,
		WorkspaceID: cfg.WorkspaceID,
		Endpoint:    cfg.Endpoint,
		UserAgent:   cfg.UserAgent,
		Timeout:     cfg.Timeout,
		RetryMax:    cfg.RetryMax,
	}
	if opts.UserAgent == "" {
		opts.UserAgent = DefaultUserAgent
	}
	return opts
}
