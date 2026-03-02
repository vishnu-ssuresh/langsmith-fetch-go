// threads.go implements the threads command flags, execution, and rendering.
package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"langsmith-fetch-go/internal/config"
	corethreads "langsmith-fetch-go/internal/core/threads"
	"langsmith-fetch-go/internal/output"
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
	return output.WriteThreadList(stdout, opts.format, threads)
}
