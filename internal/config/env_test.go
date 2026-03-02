// env_test.go verifies environment variable precedence and normalization.
package config

import "testing"

func TestLoadFromLookup_PrefersLangSmithOverLangChain(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"LANGSMITH_API_KEY":      "smith-key",
		"LANGCHAIN_API_KEY":      "chain-key",
		"LANGSMITH_WORKSPACE_ID": "smith-workspace",
		"LANGCHAIN_WORKSPACE_ID": "chain-workspace",
		"LANGCHAIN_ENDPOINT":     "https://chain.example.com",
		"LANGCHAIN_PROJECT_UUID": "chain-project-uuid",
		"LANGCHAIN_PROJECT":      "chain-project",
	}

	values := loadFromLookup(func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	})

	if values.APIKey != "smith-key" {
		t.Fatalf("APIKey = %q, want %q", values.APIKey, "smith-key")
	}
	if values.WorkspaceID != "smith-workspace" {
		t.Fatalf("WorkspaceID = %q, want %q", values.WorkspaceID, "smith-workspace")
	}
	if values.Endpoint != "https://chain.example.com" {
		t.Fatalf("Endpoint = %q, want %q", values.Endpoint, "https://chain.example.com")
	}
	if values.ProjectUUID != "chain-project-uuid" {
		t.Fatalf("ProjectUUID = %q, want %q", values.ProjectUUID, "chain-project-uuid")
	}
	if values.ProjectName != "chain-project" {
		t.Fatalf("ProjectName = %q, want %q", values.ProjectName, "chain-project")
	}
}

func TestLoadFromLookup_TrimsWhitespaceAndQuotes(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"LANGSMITH_API_KEY":      ` "test-key" `,
		"LANGSMITH_ENDPOINT":     "  https://api.example.com  ",
		"LANGSMITH_PROJECT_UUID": ` "project-uuid" `,
		"LANGSMITH_PROJECT":      ` "project-name" `,
		"LANGCHAIN_API_KEY":      "ignored",
		"LANGCHAIN_ENDPOINT":     "ignored",
		"LANGCHAIN_WORKSPACE_ID": "ignored",
	}

	values := loadFromLookup(func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	})

	if values.APIKey != "test-key" {
		t.Fatalf("APIKey = %q, want %q", values.APIKey, "test-key")
	}
	if values.Endpoint != "https://api.example.com" {
		t.Fatalf("Endpoint = %q, want %q", values.Endpoint, "https://api.example.com")
	}
	if values.ProjectUUID != "project-uuid" {
		t.Fatalf("ProjectUUID = %q, want %q", values.ProjectUUID, "project-uuid")
	}
	if values.ProjectName != "project-name" {
		t.Fatalf("ProjectName = %q, want %q", values.ProjectName, "project-name")
	}
}

func TestLoadFromLookup_EmptyWhenUnset(t *testing.T) {
	t.Parallel()

	values := loadFromLookup(func(string) (string, bool) {
		return "", false
	})

	if values.APIKey != "" ||
		values.WorkspaceID != "" ||
		values.Endpoint != "" ||
		values.ProjectUUID != "" ||
		values.ProjectName != "" {
		t.Fatalf("values = %+v, want all empty", values)
	}
}
