// file_test.go verifies config file parsing, precedence, and persistence.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromReader_ParsesKnownKeys(t *testing.T) {
	t.Parallel()

	fileContent := []byte(`
# Comment line
api-key: "file-key"
workspace-id: file-workspace
endpoint: https://example.langsmith.local
project-uuid: "project-123"
project-name: "demo-project"
default-format: pretty
`)

	values, err := loadFromReader("/tmp/config.yaml", func(string) ([]byte, error) {
		return fileContent, nil
	})
	if err != nil {
		t.Fatalf("loadFromReader() error = %v", err)
	}

	if values.APIKey != "file-key" {
		t.Fatalf("APIKey = %q, want %q", values.APIKey, "file-key")
	}
	if values.WorkspaceID != "file-workspace" {
		t.Fatalf("WorkspaceID = %q, want %q", values.WorkspaceID, "file-workspace")
	}
	if values.Endpoint != "https://example.langsmith.local" {
		t.Fatalf("Endpoint = %q, want %q", values.Endpoint, "https://example.langsmith.local")
	}
	if values.ProjectUUID != "project-123" {
		t.Fatalf("ProjectUUID = %q, want %q", values.ProjectUUID, "project-123")
	}
	if values.ProjectName != "demo-project" {
		t.Fatalf("ProjectName = %q, want %q", values.ProjectName, "demo-project")
	}
	if values.DefaultFormat != "pretty" {
		t.Fatalf("DefaultFormat = %q, want %q", values.DefaultFormat, "pretty")
	}
}

func TestLoadFromSources_EnvOverridesFile(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"LANGSMITH_API_KEY":  "env-key",
		"LANGSMITH_ENDPOINT": "https://env.example.com",
	}
	lookup := func(key string) (string, bool) {
		value, ok := env[key]
		return value, ok
	}

	fileContent := []byte(`
api-key: "file-key"
workspace-id: "file-workspace"
endpoint: "https://file.example.com"
project-name: "file-project"
default-format: "json"
`)
	readFile := func(string) ([]byte, error) {
		return fileContent, nil
	}

	values := loadFromSources(lookup, readFile, "/tmp/config.yaml")

	if values.APIKey != "env-key" {
		t.Fatalf("APIKey = %q, want %q", values.APIKey, "env-key")
	}
	if values.Endpoint != "https://env.example.com" {
		t.Fatalf("Endpoint = %q, want %q", values.Endpoint, "https://env.example.com")
	}
	if values.WorkspaceID != "file-workspace" {
		t.Fatalf("WorkspaceID = %q, want %q", values.WorkspaceID, "file-workspace")
	}
	if values.ProjectName != "file-project" {
		t.Fatalf("ProjectName = %q, want %q", values.ProjectName, "file-project")
	}
	if values.DefaultFormat != "json" {
		t.Fatalf("DefaultFormat = %q, want %q", values.DefaultFormat, "json")
	}
}

func TestLoadFromReader_MissingFileReturnsEmpty(t *testing.T) {
	t.Parallel()

	values, err := loadFromReader("/tmp/missing.yaml", func(string) ([]byte, error) {
		return nil, os.ErrNotExist
	})
	if err != nil {
		t.Fatalf("loadFromReader() error = %v", err)
	}
	if values != (Values{}) {
		t.Fatalf("values = %+v, want zero values", values)
	}
}

func TestSaveToFileAndLoadFromFile_RoundTrip(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "config.yaml")
	want := Values{
		APIKey:        "roundtrip-key",
		WorkspaceID:   "roundtrip-workspace",
		Endpoint:      "https://roundtrip.example.com",
		ProjectUUID:   "roundtrip-project-uuid",
		ProjectName:   "roundtrip-project",
		DefaultFormat: "raw",
	}

	if err := SaveToFile(path, want); err != nil {
		t.Fatalf("SaveToFile() error = %v", err)
	}

	got, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() error = %v", err)
	}

	if got != want {
		t.Fatalf("loaded values = %+v, want %+v", got, want)
	}
}
