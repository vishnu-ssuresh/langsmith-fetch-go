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
}

func TestRunConfig_UnknownSubcommand(t *testing.T) {
	t.Parallel()

	err := runConfig([]string{"set"}, &bytes.Buffer{}, &bytes.Buffer{}, Deps{}, config.Values{})
	if err == nil {
		t.Fatal("runConfig() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "unknown config subcommand") {
		t.Fatalf("runConfig() error = %v, want unknown subcommand", err)
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
