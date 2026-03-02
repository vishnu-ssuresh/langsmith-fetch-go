// traces.go implements the traces command flags, execution, and rendering.
package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"langsmith-fetch-go/internal/config"
	coretraces "langsmith-fetch-go/internal/core/traces"
	"langsmith-fetch-go/internal/files"
	"langsmith-fetch-go/internal/output"
)

type tracesOptions struct {
	projectID       string
	limit           int
	format          string
	outputFile      string
	outputDir       string
	filenamePattern string
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
	fs.StringVar(&opts.outputFile, "file", "", "Write output JSON to a single file")
	fs.StringVar(&opts.outputDir, "dir", "", "Write one JSON file per trace to a directory")
	fs.StringVar(&opts.filenamePattern, "filename-pattern", "{trace_id}.json", "File pattern for directory mode")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if opts.limit <= 0 {
		return errors.New("--limit must be > 0")
	}
	if opts.outputFile != "" && opts.outputDir != "" {
		return errors.New("--file and --dir are mutually exclusive")
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

	if opts.outputFile != "" {
		data, err := json.MarshalIndent(runs, "", "  ")
		if err != nil {
			return fmt.Errorf("encode traces output: %w", err)
		}
		if err := files.WriteFile(opts.outputFile, append(data, '\n')); err != nil {
			return err
		}
		return nil
	}

	if opts.outputDir != "" {
		if err := files.EnsureDir(opts.outputDir); err != nil {
			return err
		}
		for i, run := range runs {
			filename, err := files.ResolveFilename(opts.filenamePattern, files.NameParams{
				ID:      run.ID,
				TraceID: run.ID,
				Index:   i + 1,
			})
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(run, "", "  ")
			if err != nil {
				return fmt.Errorf("encode trace file content: %w", err)
			}
			path := filepath.Join(opts.outputDir, filename)
			if err := files.WriteFile(path, append(data, '\n')); err != nil {
				return err
			}
		}
		return nil
	}

	return output.WriteTraceSummaries(stdout, opts.format, runs)
}
