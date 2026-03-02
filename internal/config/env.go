// env.go loads runtime configuration values from environment variables.
package config

import (
	"os"
	"strings"
)

// Values contains fetch-go runtime config loaded from environment.
type Values struct {
	APIKey        string
	WorkspaceID   string
	Endpoint      string
	ProjectUUID   string
	ProjectName   string
	DefaultFormat string
}

// LoadFromEnv reads config from process environment variables.
func LoadFromEnv() Values {
	return loadFromLookup(os.LookupEnv)
}

func loadFromLookup(lookup func(string) (string, bool)) Values {
	return Values{
		APIKey: firstEnv(lookup, "LANGSMITH_API_KEY", "LANGCHAIN_API_KEY"),
		WorkspaceID: firstEnv(
			lookup,
			"LANGSMITH_WORKSPACE_ID",
			"LANGCHAIN_WORKSPACE_ID",
		),
		Endpoint: firstEnv(lookup, "LANGSMITH_ENDPOINT", "LANGCHAIN_ENDPOINT"),
		ProjectUUID: firstEnv(
			lookup,
			"LANGSMITH_PROJECT_UUID",
			"LANGCHAIN_PROJECT_UUID",
		),
		ProjectName: firstEnv(
			lookup,
			"LANGSMITH_PROJECT",
			"LANGCHAIN_PROJECT",
		),
	}
}

func firstEnv(lookup func(string) (string, bool), keys ...string) string {
	for _, key := range keys {
		value, ok := lookup(key)
		if !ok {
			continue
		}
		value = normalizeValue(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return strings.TrimSpace(value)
}
