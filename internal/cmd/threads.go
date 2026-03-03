// threads.go implements the threads command flags, execution, and rendering.
package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"langsmith-fetch-go/internal/config"
	corethreads "langsmith-fetch-go/internal/core/threads"
	"langsmith-fetch-go/internal/files"
	"langsmith-fetch-go/internal/output"
)

type threadsOptions struct {
	projectID       string
	limit           int
	lastNMinutes    int
	since           string
	maxConcurrent   int
	noProgress      bool
	format          string
	outputFile      string
	outputDir       string
	filenamePattern string
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
	fs.IntVar(&opts.limit, "n", 20, "Max threads to return (shorthand)")
	fs.IntVar(
		&opts.lastNMinutes,
		"last-n-minutes",
		unsetLastNMinutes,
		"Only fetch threads from the last N minutes",
	)
	fs.StringVar(
		&opts.since,
		"since",
		"",
		"Only fetch threads since RFC3339 timestamp (e.g., 2025-12-09T10:00:00Z)",
	)
	fs.IntVar(
		&opts.maxConcurrent,
		"max-concurrent",
		5,
		"Maximum concurrent thread fetches",
	)
	fs.BoolVar(
		&opts.noProgress,
		"no-progress",
		false,
		"Disable progress output",
	)
	fs.StringVar(
		&opts.format,
		"format",
		configuredDefaultFormat(cfg.DefaultFormat),
		"Output format: pretty|json|raw",
	)
	fs.StringVar(&opts.outputFile, "file", "", "Write output JSON to a single file")
	fs.StringVar(&opts.outputDir, "dir", "", "Write one JSON file per thread to a directory")
	fs.StringVar(&opts.filenamePattern, "filename-pattern", "{thread_id}.json", "File pattern for directory mode")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if err := validatePositiveIntFlag("limit", opts.limit); err != nil {
		return err
	}
	if err := validatePositiveIntFlag("max-concurrent", opts.maxConcurrent); err != nil {
		return err
	}
	if err := validateMutuallyExclusiveStringFlags("file", opts.outputFile, "dir", opts.outputDir); err != nil {
		return err
	}
	if err := validateOutputFormat(opts.format); err != nil {
		return err
	}

	projectID, err := resolveProjectID(opts.projectID, cfg, deps)
	if err != nil {
		return err
	}
	startTime, err := parseStartTime(opts.lastNMinutes, opts.since, time.Now)
	if err != nil {
		return err
	}

	lister, err := deps.NewThreadsLister(cfg)
	if err != nil {
		return fmt.Errorf("initialize threads service: %w", err)
	}

	showProgress := !opts.noProgress
	progress := newProgressReporter(stderr, "threads", showProgress)
	if showProgress {
		defer progress.Done()
	}

	threads, err := lister.List(context.Background(), corethreads.ListParams{
		ProjectID:     projectID,
		Limit:         opts.limit,
		StartTime:     startTime,
		MaxConcurrent: opts.maxConcurrent,
		ShowProgress:  showProgress,
		Progress:      progress.Update,
	})
	if err != nil {
		return fmt.Errorf("list threads: %w", err)
	}

	if opts.outputFile != "" {
		data, err := json.MarshalIndent(threads, "", "  ")
		if err != nil {
			return fmt.Errorf("encode threads output: %w", err)
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
		for i, thread := range threads {
			filename, err := files.ResolveFilename(opts.filenamePattern, files.NameParams{
				ID:       thread.ThreadID,
				ThreadID: thread.ThreadID,
				Index:    i + 1,
			})
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(thread.Messages, "", "  ")
			if err != nil {
				return fmt.Errorf("encode thread file content: %w", err)
			}
			path := filepath.Join(opts.outputDir, filename)
			if err := files.WriteFile(path, append(data, '\n')); err != nil {
				return err
			}
		}
		return nil
	}

	return output.WriteThreadList(stdout, opts.format, threads)
}
