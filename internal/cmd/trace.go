// trace.go implements the trace command flags, execution, and rendering.
package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"langsmith-fetch-go/internal/config"
	coresingle "langsmith-fetch-go/internal/core/single"
	"langsmith-fetch-go/internal/output"
)

type traceOptions struct {
	traceID string
	format  string
}

type traceGetter interface {
	GetMessages(context.Context, coresingle.TraceParams) ([]coresingle.Message, error)
}

func runTrace(args []string, stdout io.Writer, stderr io.Writer, deps Deps, cfg config.Values) error {
	fs := flag.NewFlagSet("trace", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts traceOptions
	fs.StringVar(&opts.traceID, "trace-id", "", "Trace ID")
	fs.StringVar(
		&opts.format,
		"format",
		configuredDefaultFormat(cfg.DefaultFormat),
		"Output format: pretty|json|raw",
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if opts.traceID == "" {
		return errors.New("--trace-id is required")
	}
	switch opts.format {
	case "pretty", "json", "raw":
	default:
		return fmt.Errorf("--format must be one of pretty|json|raw, got %q", opts.format)
	}

	getter, err := deps.NewTraceGetter(cfg)
	if err != nil {
		return fmt.Errorf("initialize trace service: %w", err)
	}

	messages, err := getter.GetMessages(context.Background(), coresingle.TraceParams{
		TraceID: opts.traceID,
	})
	if err != nil {
		return fmt.Errorf("fetch trace: %w", err)
	}
	return output.WriteTraceMessages(stdout, opts.format, messages)
}
