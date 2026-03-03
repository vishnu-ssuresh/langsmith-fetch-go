// root.go defines root command execution and top-level dispatch.
package cmd

import (
	"errors"
	"fmt"
	"io"

	"langsmith-fetch-go/internal/config"
)

const missingAPIKeyMessage = "LANGSMITH_API_KEY (or LANGCHAIN_API_KEY) is required"

// Execute runs the root CLI command.
func Execute(args []string, stdout io.Writer, stderr io.Writer, deps Deps) error {
	deps = deps.withDefaults()

	if len(args) == 0 {
		printRootUsage(stdout)
		return nil
	}

	switch args[0] {
	case "-h", "--help", "help":
		printRootUsage(stdout)
		return nil
	case "config":
		return runConfiguredRootCommand(args[1:], stdout, stderr, deps, false, runConfig)
	case "traces":
		return runConfiguredRootCommand(args[1:], stdout, stderr, deps, true, runTraces)
	case "trace":
		return runConfiguredRootCommand(args[1:], stdout, stderr, deps, true, runTrace)
	case "thread":
		return runConfiguredRootCommand(args[1:], stdout, stderr, deps, true, runThread)
	case "threads":
		return runConfiguredRootCommand(args[1:], stdout, stderr, deps, true, runThreads)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runConfiguredRootCommand(
	args []string,
	stdout io.Writer,
	stderr io.Writer,
	deps Deps,
	requireAPIKey bool,
	run func([]string, io.Writer, io.Writer, Deps, config.Values) error,
) error {
	cfg := deps.LoadConfig()
	if requireAPIKey && cfg.APIKey == "" {
		return errors.New(missingAPIKeyMessage)
	}
	return run(args, stdout, stderr, deps, cfg)
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
