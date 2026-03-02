// file.go loads and saves config values from ~/.langsmith-cli/config.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultConfigPath = "~/.langsmith-cli/config.yaml"

type readFileFn func(string) ([]byte, error)
type lookupFn func(string) (string, bool)

// Load returns effective config using env-over-file precedence.
func Load() Values {
	return loadFromSources(os.LookupEnv, os.ReadFile, "")
}

// LoadFromFile loads values from config file path (or default path if empty).
func LoadFromFile(path string) (Values, error) {
	return loadFromReader(path, os.ReadFile)
}

// SaveToFile writes config values to the config file path.
func SaveToFile(path string, values Values) error {
	resolvedPath, err := resolveConfigPath(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
		return fmt.Errorf("config: create config dir: %w", err)
	}

	content := buildConfigYAML(values)
	if err := os.WriteFile(resolvedPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("config: write config file: %w", err)
	}
	return nil
}

func loadFromSources(lookup lookupFn, readFile readFileFn, path string) Values {
	envValues := loadFromLookup(lookup)
	fileValues, err := loadFromReader(path, readFile)
	if err != nil {
		return envValues
	}

	// Env wins over file values.
	out := envValues
	if out.APIKey == "" {
		out.APIKey = fileValues.APIKey
	}
	if out.WorkspaceID == "" {
		out.WorkspaceID = fileValues.WorkspaceID
	}
	if out.Endpoint == "" {
		out.Endpoint = fileValues.Endpoint
	}
	if out.ProjectUUID == "" {
		out.ProjectUUID = fileValues.ProjectUUID
	}
	if out.ProjectName == "" {
		out.ProjectName = fileValues.ProjectName
	}
	if out.DefaultFormat == "" {
		out.DefaultFormat = fileValues.DefaultFormat
	}
	return out
}

func loadFromReader(path string, readFile readFileFn) (Values, error) {
	resolvedPath, err := resolveConfigPath(path)
	if err != nil {
		return Values{}, err
	}

	data, err := readFile(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Values{}, nil
		}
		return Values{}, fmt.Errorf("config: read config file: %w", err)
	}

	values := parseFlatYAML(data)
	return Values{
		APIKey: firstMapValue(values,
			"api-key",
			"api_key",
			"langsmith-api-key",
			"langsmith_api_key",
		),
		WorkspaceID: firstMapValue(values,
			"workspace-id",
			"workspace_id",
			"langsmith-workspace-id",
			"langsmith_workspace_id",
		),
		Endpoint: firstMapValue(values,
			"endpoint",
			"base-url",
			"base_url",
			"langsmith-endpoint",
			"langsmith_endpoint",
		),
		ProjectUUID: firstMapValue(values,
			"project-uuid",
			"project_uuid",
			"langsmith-project-uuid",
			"langsmith_project_uuid",
		),
		ProjectName: firstMapValue(values,
			"project-name",
			"project_name",
			"langsmith-project",
			"langsmith_project",
		),
		DefaultFormat: firstMapValue(values,
			"default-format",
			"default_format",
		),
	}, nil
}

func resolveConfigPath(path string) (string, error) {
	if path == "" {
		path = defaultConfigPath
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("config: resolve home dir: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path, nil
}

func parseFlatYAML(data []byte) map[string]string {
	out := make(map[string]string)
	lines := strings.Split(string(data), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexRune(line, ':')
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		value := normalizeValue(line[idx+1:])
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func firstMapValue(values map[string]string, keys ...string) string {
	for _, key := range keys {
		value, ok := values[key]
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

func buildConfigYAML(values Values) string {
	var b strings.Builder
	writeYAMLLine(&b, "api-key", values.APIKey)
	writeYAMLLine(&b, "workspace-id", values.WorkspaceID)
	writeYAMLLine(&b, "endpoint", values.Endpoint)
	writeYAMLLine(&b, "project-uuid", values.ProjectUUID)
	writeYAMLLine(&b, "project-name", values.ProjectName)
	writeYAMLLine(&b, "default-format", values.DefaultFormat)
	return b.String()
}

func writeYAMLLine(b *strings.Builder, key, value string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return
	}
	// Quote to keep parsing simple and preserve special characters.
	fmt.Fprintf(b, "%s: %q\n", key, value)
}
