package app

import "testing"

func TestNewClientFromEnv_WithAPIKey(t *testing.T) {
	t.Setenv("LANGSMITH_API_KEY", "test-api-key")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("NewClientFromEnv() error = %v", err)
	}
	if client == nil {
		t.Fatal("NewClientFromEnv() returned nil client")
	}
}
