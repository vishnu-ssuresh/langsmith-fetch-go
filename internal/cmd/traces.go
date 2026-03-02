// traces.go implements the traces command flags, execution, and rendering.
package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"langsmith-fetch-go/internal/config"
	coretraces "langsmith-fetch-go/internal/core/traces"
	"langsmith-fetch-go/internal/output"
)

type tracesOptions struct {
	projectID string
	limit     int
	format    string
}

type tracesLister interface {
	List(context.Context, coretraces.ListParams) ([]coretraces.Summary, error)
}

func runTraces(args []string, stdout io.Writer, stderr io.Writer, deps Deps, cfg config.Values) error {
	fs := flag.NewFlagSet("traces", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts tracesOptions
	fs.StringVar(&opts.projectID, "project-id", "", "Project UUID")
	fs.StringVar(&opts.projectID, "project-uuid", "", "Project UUID")
	fs.IntVar(&opts.limit, "limit", 20, "Max traces to return")
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

	lister, err := deps.NewTracesLister(cfg)
	if err != nil {
		return fmt.Errorf("initialize traces service: %w", err)
	}

	runs, err := lister.List(context.Background(), coretraces.ListParams{
		ProjectID: projectID,
		Limit:     opts.limit,
	})
	if err != nil {
		return fmt.Errorf("list traces: %w", err)
	}
	return output.WriteTraceSummaries(stdout, opts.format, runs)
}
