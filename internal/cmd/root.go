// root.go defines root command execution and top-level dispatch.
package cmd

import (
	"errors"
	"fmt"
	"io"
)

const missingAPIKeyMessage = "LANGSMITH_API_KEY (or LANGCHAIN_API_KEY) is required"

// Execute runs the root CLI command.
func Execute(args []string, stdout io.Writer, stderr io.Writer, deps Deps) error {
	_ = stderr // command-specific handlers will use this in later steps.
	deps = deps.withDefaults()

	cfg := deps.LoadConfig()
	if cfg.APIKey == "" {
		return errors.New(missingAPIKeyMessage)
	}

	if len(args) == 0 {
		printRootUsage(stdout)
		return nil
	}

	switch args[0] {
	case "-h", "--help", "help":
		printRootUsage(stdout)
		return nil
	case "traces":
		return runTraces(args[1:], stdout, stderr, deps, cfg)
	case "trace", "thread", "threads", "config":
		return fmt.Errorf("command %q not implemented yet", args[0])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "langsmith-fetch-go")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  langsmith-fetch <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  trace      Fetch one trace")
	fmt.Fprintln(w, "  traces     List traces")
	fmt.Fprintln(w, "  thread     Fetch one thread")
	fmt.Fprintln(w, "  threads    List threads")
	fmt.Fprintln(w, "  config     Show CLI configuration")
}
