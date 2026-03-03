// trace.go implements the trace command flags, execution, and rendering.
package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"time"

	"langsmith-fetch-go/internal/config"
	coresingle "langsmith-fetch-go/internal/core/single"
	"langsmith-fetch-go/internal/files"
	langsmithfeedback "langsmith-fetch-go/internal/langsmith/feedback"
	"langsmith-fetch-go/internal/output"
)

type traceOptions struct {
	traceID         string
	format          string
	outputFile      string
	includeMetadata bool
	includeFeedback bool
}

type traceGetter interface {
	GetMessages(context.Context, coresingle.TraceParams) ([]coresingle.Message, error)
	GetRun(context.Context, coresingle.TraceParams) (coresingle.Run, error)
}

type traceFeedbackAccessor interface {
	ListByRuns(context.Context, langsmithfeedback.ListParams) ([]langsmithfeedback.Item, error)
}

type traceOutput struct {
	TraceID  string                   `json:"trace_id"`
	Messages []coresingle.Message     `json:"messages"`
	Metadata *traceMetadata           `json:"metadata,omitempty"`
	Feedback []langsmithfeedback.Item `json:"feedback,omitempty"`
}

type traceMetadata struct {
	Status        string          `json:"status,omitempty"`
	StartTime     string          `json:"start_time,omitempty"`
	EndTime       string          `json:"end_time,omitempty"`
	DurationMS    *int64          `json:"duration_ms,omitempty"`
	CustomMeta    json.RawMessage `json:"custom_metadata,omitempty"`
	TokenUsage    tokenUsage      `json:"token_usage"`
	Costs         costs           `json:"costs"`
	FirstTokenAt  string          `json:"first_token_time,omitempty"`
	FeedbackStats json.RawMessage `json:"feedback_stats,omitempty"`
}

type tokenUsage struct {
	PromptTokens     *int `json:"prompt_tokens,omitempty"`
	CompletionTokens *int `json:"completion_tokens,omitempty"`
	TotalTokens      *int `json:"total_tokens,omitempty"`
}

type costs struct {
	PromptCost     *float64 `json:"prompt_cost,omitempty"`
	CompletionCost *float64 `json:"completion_cost,omitempty"`
	TotalCost      *float64 `json:"total_cost,omitempty"`
}

func runTrace(args []string, stdout io.Writer, stderr io.Writer, deps Deps, cfg config.Values) error {
	fs := flag.NewFlagSet("trace", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var opts traceOptions
	leadingTraceID, parseArgs := popLeadingPositionalArg(args)
	fs.StringVar(&opts.traceID, "trace-id", "", "Trace ID")
	fs.StringVar(
		&opts.format,
		"format",
		configuredDefaultFormat(cfg.DefaultFormat),
		"Output format: pretty|json|raw",
	)
	fs.StringVar(&opts.outputFile, "file", "", "Write output to a file instead of stdout")
	fs.BoolVar(
		&opts.includeMetadata,
		"include-metadata",
		false,
		"Include trace metadata fields",
	)
	fs.BoolVar(
		&opts.includeFeedback,
		"include-feedback",
		false,
		"Include trace feedback (extra API call)",
	)
	if err := fs.Parse(parseArgs); err != nil {
		return err
	}

	traceID, err := resolveRequiredIDFromArgs(opts.traceID, leadingTraceID, fs.Args(), "trace-id")
	if err != nil {
		return err
	}
	opts.traceID = traceID
	if err := validateOutputFormat(opts.format); err != nil {
		return err
	}

	getter, err := deps.NewTraceGetter(cfg)
	if err != nil {
		return fmt.Errorf("initialize trace service: %w", err)
	}

	if opts.includeMetadata || opts.includeFeedback {
		details, err := getTraceWithDetails(context.Background(), getter, deps, cfg, opts)
		if err != nil {
			return err
		}
		if opts.outputFile != "" {
			var out bytes.Buffer
			if err := writeTraceDetails(&out, opts.format, details); err != nil {
				return err
			}
			return files.WriteFile(opts.outputFile, out.Bytes())
		}
		return writeTraceDetails(stdout, opts.format, details)
	}

	messages, err := getter.GetMessages(context.Background(), coresingle.TraceParams{
		TraceID: opts.traceID,
	})
	if err != nil {
		return fmt.Errorf("fetch trace: %w", err)
	}

	if opts.outputFile != "" {
		var out bytes.Buffer
		if err := output.WriteTraceMessages(&out, opts.format, messages); err != nil {
			return err
		}
		return files.WriteFile(opts.outputFile, out.Bytes())
	}

	return output.WriteTraceMessages(stdout, opts.format, messages)
}

func getTraceWithDetails(
	ctx context.Context,
	getter traceGetter,
	deps Deps,
	cfg config.Values,
	opts traceOptions,
) (traceOutput, error) {
	run, err := getter.GetRun(ctx, coresingle.TraceParams{TraceID: opts.traceID})
	if err != nil {
		return traceOutput{}, fmt.Errorf("fetch trace: %w", err)
	}

	messages := run.Messages
	if len(messages) == 0 {
		messages = run.Outputs.Messages
	}
	if len(messages) == 0 {
		messages = []coresingle.Message{}
	}

	result := traceOutput{
		TraceID:  opts.traceID,
		Messages: messages,
	}
	if run.ID != "" {
		result.TraceID = run.ID
	}

	if opts.includeMetadata {
		metadata := toTraceMetadata(run)
		result.Metadata = &metadata
	}
	if opts.includeFeedback {
		if deps.NewFeedbackAccessor == nil {
			return traceOutput{}, fmt.Errorf("initialize feedback accessor: dependency not configured")
		}
		feedbackAccessor, err := deps.NewFeedbackAccessor(cfg)
		if err != nil {
			return traceOutput{}, fmt.Errorf("initialize feedback accessor: %w", err)
		}
		items, err := feedbackAccessor.ListByRuns(ctx, langsmithfeedback.ListParams{
			RunIDs: []string{result.TraceID},
		})
		if err != nil {
			return traceOutput{}, fmt.Errorf("fetch trace feedback: %w", err)
		}
		result.Feedback = items
	}
	return result, nil
}

func writeTraceDetails(w io.Writer, format string, details traceOutput) error {
	switch format {
	case "json", "raw":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(details)
	case "pretty":
		if _, err := fmt.Fprintf(w, "Trace: %s\n", details.TraceID); err != nil {
			return err
		}
		if details.Metadata != nil {
			if details.Metadata.Status != "" {
				if _, err := fmt.Fprintf(w, "Status: %s\n", details.Metadata.Status); err != nil {
					return err
				}
			}
			if details.Metadata.DurationMS != nil {
				if _, err := fmt.Fprintf(w, "Duration(ms): %d\n", *details.Metadata.DurationMS); err != nil {
					return err
				}
			}
		}
		if len(details.Feedback) > 0 {
			if _, err := fmt.Fprintf(w, "Feedback: %d item(s)\n", len(details.Feedback)); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		return output.WriteTraceMessages(w, "pretty", details.Messages)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func toTraceMetadata(run coresingle.Run) traceMetadata {
	return traceMetadata{
		Status:     run.Status,
		StartTime:  run.StartTime,
		EndTime:    run.EndTime,
		DurationMS: parseDurationMilliseconds(run.StartTime, run.EndTime),
		CustomMeta: run.Extra.Metadata,
		TokenUsage: tokenUsage{
			PromptTokens:     run.PromptTokens,
			CompletionTokens: run.CompletionTokens,
			TotalTokens:      run.TotalTokens,
		},
		Costs: costs{
			PromptCost:     run.PromptCost,
			CompletionCost: run.CompletionCost,
			TotalCost:      run.TotalCost,
		},
		FirstTokenAt:  run.FirstTokenTime,
		FeedbackStats: run.FeedbackStats,
	}
}

func parseDurationMilliseconds(startTime string, endTime string) *int64 {
	if startTime == "" || endTime == "" {
		return nil
	}
	start, err := time.Parse(time.RFC3339, startTime)
	if err != nil {
		return nil
	}
	end, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return nil
	}
	durationMS := end.Sub(start).Milliseconds()
	return &durationMS
}
