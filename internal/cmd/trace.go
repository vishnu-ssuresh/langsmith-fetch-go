// trace.go implements the trace command flags, execution, and rendering.
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
	coresingle "langsmith-fetch-go/internal/core/single"
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
	fs.StringVar(&opts.format, "format", "pretty", "Output format: pretty|json|raw")
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

	switch opts.format {
	case "json", "raw":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(messages)
	case "pretty":
		return printTracePretty(stdout, messages)
	default:
		return fmt.Errorf("unsupported format %q", opts.format)
	}
}

func printTracePretty(w io.Writer, messages []coresingle.Message) error {
	if len(messages) == 0 {
		fmt.Fprintln(w, "No trace messages found.")
		return nil
	}

	for i, message := range messages {
		line := strings.TrimSpace(string(message))
		if _, err := fmt.Fprintf(w, "[%d] %s\n", i+1, line); err != nil {
			return err
		}
	}
	return nil
}
