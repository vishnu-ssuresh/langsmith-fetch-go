// threads.go implements the threads command flags, execution, and rendering.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"langsmith-fetch-go/internal/config"
	corethreads "langsmith-fetch-go/internal/core/threads"
)

type threadsOptions struct {
	projectID string
	limit     int
	format    string
}

type threadsLister interface {
	List(context.Context, corethreads.ListParams) ([]corethreads.ThreadData, error)
}

func runThreads(args []string, stdout io.Writer, stderr io.Writer, deps Deps, cfg config.Values) error {
	fs := flag.NewFlagSet("threads", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts threadsOptions
	fs.StringVar(&opts.projectID, "project-id", "", "Project UUID")
	fs.StringVar(&opts.projectID, "project-uuid", "", "Project UUID")
	fs.IntVar(&opts.limit, "limit", 20, "Max threads to return")
	fs.StringVar(&opts.format, "format", "pretty", "Output format: pretty|json|raw")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if opts.limit <= 0 {
		return errors.New("--limit must be > 0")
	}
	switch opts.format {
	case "pretty", "json", "raw":
	default:
		return fmt.Errorf("--format must be one of pretty|json|raw, got %q", opts.format)
	}

	projectID, err := resolveProjectID(opts.projectID, cfg, deps)
	if err != nil {
		return err
	}

	lister, err := deps.NewThreadsLister(cfg)
	if err != nil {
		return fmt.Errorf("initialize threads service: %w", err)
	}

	threads, err := lister.List(context.Background(), corethreads.ListParams{
		ProjectID: projectID,
		Limit:     opts.limit,
	})
	if err != nil {
		return fmt.Errorf("list threads: %w", err)
	}

	switch opts.format {
	case "json", "raw":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(threads)
	case "pretty":
		return printThreadsPretty(stdout, threads)
	default:
		return fmt.Errorf("unsupported format %q", opts.format)
	}
}

func printThreadsPretty(w io.Writer, threads []corethreads.ThreadData) error {
	if len(threads) == 0 {
		fmt.Fprintln(w, "No threads found.")
		return nil
	}

	for _, thread := range threads {
		if _, err := fmt.Fprintf(w, "Thread: %s\n", thread.ThreadID); err != nil {
			return err
		}
		for i, message := range thread.Messages {
			line := strings.TrimSpace(string(message))
			if _, err := fmt.Fprintf(w, "  [%d] %s\n", i+1, line); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}
