// config_show_test.go verifies config command dispatch and output formatting.
package cmd

import (
	"bytes"
	"strings"
	"testing"

	"langsmith-fetch-go/internal/config"
)

func TestRunConfig_Show(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := runConfig(
		[]string{"show"},
		&out,
		&bytes.Buffer{},
		Deps{},
		config.Values{
			APIKey:        "test-api-key-secret",
			WorkspaceID:   "workspace-123",
			Endpoint:      "https://api.example.com",
			ProjectUUID:   "project-uuid-123",
			ProjectName:   "my-project",
			DefaultFormat: "json",
		},
	)
	if err != nil {
		t.Fatalf("runConfig() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"Current configuration:",
		"api_key:",
		"test-api...",
		"workspace_id: workspace-123",
		"endpoint: https://api.example.com",
		"project_uuid: project-uuid-123",
		"project_name: my-project",
		"default_format: json",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want to contain %q", got, want)
		}
	}
}

func TestRunConfig_Help(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := runConfig([]string{"help"}, &out, &bytes.Buffer{}, Deps{}, config.Values{})
	if err != nil {
		t.Fatalf("runConfig() error = %v", err)
	}
	if !strings.Contains(out.String(), "langsmith-fetch config show") {
		t.Fatalf("stdout = %q, want config usage", out.String())
	}
	if !strings.Contains(out.String(), "langsmith-fetch config set <key> <value>") {
		t.Fatalf("stdout = %q, want config set usage", out.String())
	}
}

func TestRunConfig_UnknownSubcommand(t *testing.T) {
	t.Parallel()

	err := runConfig([]string{"delete"}, &bytes.Buffer{}, &bytes.Buffer{}, Deps{}, config.Values{})
	if err == nil {
		t.Fatal("runConfig() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "unknown config subcommand") {
		t.Fatalf("runConfig() error = %v, want unknown subcommand", err)
	}
}

func TestRunConfig_Set(t *testing.T) {
	loadFn := func(string) (config.Values, error) {
		return config.Values{
			ProjectName: "old-project",
		}, nil
	}

	var saved config.Values
	saveFn := func(_ string, values config.Values) error {
		saved = values
		return nil
	}

	var out bytes.Buffer
	err := runConfigSetWithStore([]string{"project-uuid", "project-123"}, &out, loadFn, saveFn)
	if err != nil {
		t.Fatalf("runConfigSetWithStore() error = %v", err)
	}
	if saved.ProjectUUID != "project-123" {
		t.Fatalf("saved.ProjectUUID = %q, want %q", saved.ProjectUUID, "project-123")
	}
	if saved.ProjectName != "old-project" {
		t.Fatalf("saved.ProjectName = %q, want %q", saved.ProjectName, "old-project")
	}
	if !strings.Contains(out.String(), "Updated project-uuid in config.") {
		t.Fatalf("stdout = %q, want update message", out.String())
	}
}

func TestRunConfig_SetRejectsBadKey(t *testing.T) {
	t.Parallel()

	err := runConfig(
		[]string{"set", "bad-key", "value"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{},
		config.Values{},
	)
	if err == nil || !strings.Contains(err.Error(), "unsupported config key") {
		t.Fatalf("runConfig() error = %v, want unsupported config key error", err)
	}
}

func TestRunConfig_SetValidatesDefaultFormat(t *testing.T) {
	loadFn := func(string) (config.Values, error) {
		return config.Values{}, nil
	}
	saveFn := func(string, config.Values) error {
		t.Fatal("saveConfigToFile() called unexpectedly")
		return nil
	}

	err := runConfigSetWithStore([]string{"default-format", "xml"}, &bytes.Buffer{}, loadFn, saveFn)
	if err == nil || !strings.Contains(err.Error(), "default-format must be one of") {
		t.Fatalf("runConfig() error = %v, want default-format validation error", err)
	}
}

func TestRunConfig_SetSupportsUnderscoreKey(t *testing.T) {
	loadFn := func(string) (config.Values, error) {
		return config.Values{}, nil
	}
	var saved config.Values
	saveFn := func(_ string, values config.Values) error {
		saved = values
		return nil
	}

	err := runConfigSetWithStore([]string{"project_uuid", "project-123"}, &bytes.Buffer{}, loadFn, saveFn)
	if err != nil {
		t.Fatalf("runConfig() error = %v", err)
	}
	if saved.ProjectUUID != "project-123" {
		t.Fatalf("saved.ProjectUUID = %q, want %q", saved.ProjectUUID, "project-123")
	}
}

func TestRunConfig_SetUsageError(t *testing.T) {
	t.Parallel()

	err := runConfig(
		[]string{"set", "project-uuid"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		Deps{},
		config.Values{},
	)
	if err == nil || !strings.Contains(err.Error(), "usage: langsmith-fetch config set") {
		t.Fatalf("runConfig() error = %v, want usage error", err)
	}
}

func TestMaskSecret(t *testing.T) {
	t.Parallel()

	if got := maskSecret(""); got != "(not set)" {
		t.Fatalf("maskSecret(\"\") = %q, want %q", got, "(not set)")
	}
	if got := maskSecret("short"); got != "********" {
		t.Fatalf("maskSecret(short) = %q, want %q", got, "********")
	}
	if got := maskSecret("abcdefghi"); got != "abcdefgh..." {
		t.Fatalf("maskSecret(long) = %q, want %q", got, "abcdefgh...")
	}
}
