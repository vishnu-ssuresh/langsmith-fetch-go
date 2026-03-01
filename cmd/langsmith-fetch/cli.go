package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

type tracesOptions struct {
	projectID string
	traceID   string
	limit     int
	format    string
}

func execute(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) == 0 {
		printRootUsage(stdout)
		return nil
	}

	switch args[0] {
	case "traces":
		return runTraces(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		printRootUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runTraces(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("traces", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts tracesOptions
	fs.StringVar(&opts.projectID, "project-id", "", "Project UUID")
	fs.StringVar(&opts.traceID, "trace-id", "", "Trace UUID")
	fs.IntVar(&opts.limit, "limit", 20, "Max traces to return")
	fs.StringVar(&opts.format, "format", "pretty", "Output format: pretty|json|raw")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if opts.projectID == "" {
		return errors.New("--project-id is required")
	}
	if opts.limit <= 0 {
		return errors.New("--limit must be > 0")
	}
	switch opts.format {
	case "pretty", "json", "raw":
	default:
		return fmt.Errorf("--format must be one of pretty|json|raw, got %q", opts.format)
	}

	fmt.Fprintf(stdout, "traces command parsed (project_id=%s limit=%d format=%s)\n", opts.projectID, opts.limit, opts.format)
	return nil
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "langsmith-fetch-go")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  langsmith-fetch <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  traces    List traces (skeleton)")
}
