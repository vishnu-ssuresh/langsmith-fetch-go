// thread.go implements the thread command flags, execution, and rendering.
package cmd

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"langsmith-fetch-go/internal/config"
	coresingle "langsmith-fetch-go/internal/core/single"
	"langsmith-fetch-go/internal/files"
	"langsmith-fetch-go/internal/output"
)

type threadOptions struct {
	projectID  string
	threadID   string
	format     string
	outputFile string
}

type threadGetter interface {
	GetMessages(context.Context, coresingle.ThreadParams) ([]coresingle.ThreadMessage, error)
}

func runThread(args []string, stdout io.Writer, stderr io.Writer, deps Deps, cfg config.Values) error {
	fs := flag.NewFlagSet("thread", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts threadOptions
	var leadingThreadID string
	parseArgs := args
	if len(parseArgs) > 0 && !strings.HasPrefix(parseArgs[0], "-") {
		leadingThreadID = parseArgs[0]
		parseArgs = parseArgs[1:]
	}
	fs.StringVar(&opts.projectID, "project-id", "", "Project UUID")
	fs.StringVar(&opts.projectID, "project-uuid", "", "Project UUID")
	fs.StringVar(&opts.threadID, "thread-id", "", "Thread ID")
	fs.StringVar(
		&opts.format,
		"format",
		configuredDefaultFormat(cfg.DefaultFormat),
		"Output format: pretty|json|raw",
	)
	fs.StringVar(&opts.outputFile, "file", "", "Write output to a file instead of stdout")
	if err := fs.Parse(parseArgs); err != nil {
		return err
	}

	rest := fs.Args()
	if opts.threadID == "" {
		if leadingThreadID != "" {
			opts.threadID = leadingThreadID
		} else if len(rest) > 0 {
			opts.threadID = rest[0]
			rest = rest[1:]
		}
	}
	if len(rest) > 0 {
		return fmt.Errorf("unexpected positional arguments: %v", rest)
	}

	if opts.threadID == "" {
		return errors.New("--thread-id is required")
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

	getter, err := deps.NewThreadGetter(cfg)
	if err != nil {
		return fmt.Errorf("initialize threads service: %w", err)
	}

	messages, err := getter.GetMessages(context.Background(), coresingle.ThreadParams{
		ThreadID:  opts.threadID,
		ProjectID: projectID,
	})
	if err != nil {
		return fmt.Errorf("fetch thread: %w", err)
	}

	if opts.outputFile != "" {
		var out bytes.Buffer
		if err := output.WriteThreadMessages(&out, opts.format, messages); err != nil {
			return err
		}
		return files.WriteFile(opts.outputFile, out.Bytes())
	}

	return output.WriteThreadMessages(stdout, opts.format, messages)
}
