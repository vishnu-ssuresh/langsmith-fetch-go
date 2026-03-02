// config_show.go implements the config command and config show subcommand.
package cmd

import (
	"fmt"
	"io"
	"strings"

	"langsmith-fetch-go/internal/config"
)

func runConfig(args []string, stdout io.Writer, _ io.Writer, _ Deps, cfg config.Values) error {
	if len(args) == 0 {
		printConfigUsage(stdout)
		return nil
	}

	switch args[0] {
	case "-h", "--help", "help":
		printConfigUsage(stdout)
		return nil
	case "show":
		return runConfigShow(stdout, cfg)
	case "set":
		return runConfigSet(args[1:], stdout)
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

func runConfigShow(w io.Writer, cfg config.Values) error {
	_, err := fmt.Fprintf(
		w,
		"Current configuration:\n  api_key: %s\n  workspace_id: %s\n  endpoint: %s\n  project_uuid: %s\n  project_name: %s\n  default_format: %s\n",
		maskSecret(cfg.APIKey),
		blankAsNotSet(cfg.WorkspaceID),
		blankAsNotSet(cfg.Endpoint),
		blankAsNotSet(cfg.ProjectUUID),
		blankAsNotSet(cfg.ProjectName),
		blankAsNotSet(cfg.DefaultFormat),
	)
	return err
}

func printConfigUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  langsmith-fetch config show")
	fmt.Fprintln(w, "  langsmith-fetch config set <key> <value>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Keys:")
	fmt.Fprintln(w, "  api-key | workspace-id | endpoint | project-uuid | project-name | default-format")
}

func blankAsNotSet(value string) string {
	if value == "" {
		return "(not set)"
	}
	return value
}

func maskSecret(secret string) string {
	if secret == "" {
		return "(not set)"
	}
	if len(secret) <= 8 {
		return "********"
	}
	return secret[:8] + "..."
}

func runConfigSet(args []string, stdout io.Writer) error {
	return runConfigSetWithStore(args, stdout, config.LoadFromFile, config.SaveToFile)
}

func runConfigSetWithStore(
	args []string,
	stdout io.Writer,
	loadFn func(string) (config.Values, error),
	saveFn func(string, config.Values) error,
) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: langsmith-fetch config set <key> <value>")
	}

	key := normalizeConfigKey(args[0])
	value := strings.TrimSpace(args[1])

	values, err := loadFn("")
	if err != nil {
		return err
	}

	if err := setConfigValue(&values, key, value); err != nil {
		return err
	}
	if err := saveFn("", values); err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "Updated %s in config.\n", key)
	return err
}

func normalizeConfigKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	return strings.ReplaceAll(key, "_", "-")
}

func setConfigValue(values *config.Values, key string, value string) error {
	switch key {
	case "api-key":
		values.APIKey = value
	case "workspace-id":
		values.WorkspaceID = value
	case "endpoint":
		values.Endpoint = value
	case "project-uuid":
		values.ProjectUUID = value
	case "project-name":
		values.ProjectName = value
	case "default-format":
		if value != "" && value != "pretty" && value != "json" && value != "raw" {
			return fmt.Errorf("default-format must be one of pretty|json|raw")
		}
		values.DefaultFormat = value
	default:
		return fmt.Errorf("unsupported config key %q", key)
	}
	return nil
}
