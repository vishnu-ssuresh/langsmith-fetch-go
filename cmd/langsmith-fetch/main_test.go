package main

import "testing"

func TestRun_ReturnsErrorWhenAPIKeyMissing(t *testing.T) {
	t.Setenv("LANGSMITH_API_KEY", "")
	t.Setenv("LANGCHAIN_API_KEY", "")

	err := runWithArgs(nil)
	if err == nil {
		t.Fatal("run() error = nil, want non-nil")
	}
}

func TestRun_SucceedsWhenAPIKeyPresent(t *testing.T) {
	t.Setenv("LANGSMITH_API_KEY", "test-api-key")

	err := runWithArgs(nil)
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
}
