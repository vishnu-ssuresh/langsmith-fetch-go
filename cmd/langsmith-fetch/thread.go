package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"langsmith-fetch-go/internal/app"
	"langsmith-fetch-go/internal/core/threads"
)

type threadOptions struct {
	projectID string
	threadID  string
	format    string
}

type threadGetter interface {
	GetMessages(context.Context, threads.GetParams) ([]threads.Message, error)
}

var newThreadGetter = defaultNewThreadGetter

func runThread(args []string, stdout io.Writer, stderr io.Writer) error {
	fs := flag.NewFlagSet("thread", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts threadOptions
	fs.StringVar(&opts.projectID, "project-id", "", "Project UUID")
	fs.StringVar(&opts.threadID, "thread-id", "", "Thread ID")
	fs.StringVar(&opts.format, "format", "pretty", "Output format: pretty|json|raw")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if opts.projectID == "" {
		return errors.New("--project-id is required")
	}
	if opts.threadID == "" {
		return errors.New("--thread-id is required")
	}
	switch opts.format {
	case "pretty", "json", "raw":
	default:
		return fmt.Errorf("--format must be one of pretty|json|raw, got %q", opts.format)
	}

	getter, err := newThreadGetter()
	if err != nil {
		return fmt.Errorf("initialize threads service: %w", err)
	}

	messages, err := getter.GetMessages(context.Background(), threads.GetParams{
		ThreadID:  opts.threadID,
		ProjectID: opts.projectID,
	})
	if err != nil {
		return fmt.Errorf("fetch thread: %w", err)
	}

	switch opts.format {
	case "json", "raw":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(messages)
	case "pretty":
		return printThreadPretty(stdout, messages)
	default:
		return fmt.Errorf("unsupported format %q", opts.format)
	}
}

func defaultNewThreadGetter() (threadGetter, error) {
	client, err := app.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return threads.New(client)
}

func printThreadPretty(w io.Writer, messages []threads.Message) error {
	if len(messages) == 0 {
		fmt.Fprintln(w, "No thread messages found.")
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
