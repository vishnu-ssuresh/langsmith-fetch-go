package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"langsmith-fetch-go/internal/app"
	"langsmith-fetch-go/internal/core/traces"
)

type tracesOptions struct {
	projectID string
	traceID   string
	limit     int
	format    string
}

type tracesLister interface {
	List(context.Context, traces.ListParams) ([]traces.Summary, error)
}

var newTracesLister = defaultNewTracesLister

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

	lister, err := newTracesLister()
	if err != nil {
		return fmt.Errorf("initialize traces service: %w", err)
	}

	runs, err := lister.List(context.Background(), traces.ListParams{
		ProjectID: opts.projectID,
		Limit:     opts.limit,
	})
	if err != nil {
		return fmt.Errorf("list traces: %w", err)
	}

	switch opts.format {
	case "json", "raw":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(runs)
	case "pretty":
		return printTracesPretty(stdout, runs)
	default:
		return fmt.Errorf("unsupported format %q", opts.format)
	}
}

func defaultNewTracesLister() (tracesLister, error) {
	client, err := app.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return traces.New(client)
}

func printTracesPretty(w io.Writer, runs []traces.Summary) error {
	if len(runs) == 0 {
		fmt.Fprintln(w, "No traces found.")
		return nil
	}
	for _, run := range runs {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", run.ID, run.Name, run.StartTime); err != nil {
			return err
		}
	}
	return nil
}

func printRootUsage(w io.Writer) {
	fmt.Fprintln(w, "langsmith-fetch-go")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  langsmith-fetch <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commands:")
	fmt.Fprintln(w, "  traces    List traces")
}
