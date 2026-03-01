package client

import (
	"testing"
	"time"
)

func TestBuildOptions_MapsFields(t *testing.T) {
	t.Parallel()

	cfg := Config{
		APIKey:      "api-key",
		WorkspaceID: "workspace-id",
		Endpoint:    "https://api.example.com",
		UserAgent:   "custom-agent",
		Timeout:     10 * time.Second,
		RetryMax:    5,
	}

	opts := buildOptions(cfg)
	if opts.APIKey != cfg.APIKey {
		t.Fatalf("APIKey = %q, want %q", opts.APIKey, cfg.APIKey)
	}
	if opts.WorkspaceID != cfg.WorkspaceID {
		t.Fatalf("WorkspaceID = %q, want %q", opts.WorkspaceID, cfg.WorkspaceID)
	}
	if opts.Endpoint != cfg.Endpoint {
		t.Fatalf("Endpoint = %q, want %q", opts.Endpoint, cfg.Endpoint)
	}
	if opts.UserAgent != cfg.UserAgent {
		t.Fatalf("UserAgent = %q, want %q", opts.UserAgent, cfg.UserAgent)
	}
	if opts.Timeout != cfg.Timeout {
		t.Fatalf("Timeout = %s, want %s", opts.Timeout, cfg.Timeout)
	}
	if opts.RetryMax != cfg.RetryMax {
		t.Fatalf("RetryMax = %d, want %d", opts.RetryMax, cfg.RetryMax)
	}
}

func TestBuildOptions_DefaultUserAgent(t *testing.T) {
	t.Parallel()

	opts := buildOptions(Config{})
	if opts.UserAgent != DefaultUserAgent {
		t.Fatalf("UserAgent = %q, want %q", opts.UserAgent, DefaultUserAgent)
	}
}

func TestNew_CreatesClient(t *testing.T) {
	t.Parallel()

	client, err := New(Config{APIKey: "test-api-key"})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client == nil {
		t.Fatal("New() returned nil client")
	}
}
