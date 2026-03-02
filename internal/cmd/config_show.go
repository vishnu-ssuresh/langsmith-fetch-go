// config_show.go implements the config command and config show subcommand.
package cmd

import (
	"fmt"
	"io"

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
