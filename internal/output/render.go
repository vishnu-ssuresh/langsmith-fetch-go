// render.go centralizes raw/json/pretty output rendering for CLI commands.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	coresingle "langsmith-fetch-go/internal/core/single"
	corethreads "langsmith-fetch-go/internal/core/threads"
	coretraces "langsmith-fetch-go/internal/core/traces"
)

// WriteTraceMessages renders single-trace messages in the requested format.
func WriteTraceMessages(w io.Writer, format string, messages []coresingle.Message) error {
	switch format {
	case "json", "raw":
		return writeJSONTraceMessages(w, messages)
	case "pretty":
		return writePrettyMessages(w, messages, "No trace messages found.")
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

// WriteThreadMessages renders single-thread messages in the requested format.
func WriteThreadMessages(w io.Writer, format string, messages []coresingle.ThreadMessage) error {
	switch format {
	case "json", "raw":
		return writeJSONThreadMessages(w, messages)
	case "pretty":
		return writePrettyMessages(w, messages, "No thread messages found.")
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

// WriteTraceSummaries renders trace summaries in the requested format.
func WriteTraceSummaries(w io.Writer, format string, runs []coretraces.Summary) error {
	switch format {
	case "json", "raw":
		return writeJSONTraceSummaries(w, runs)
	case "pretty":
		return writePrettyTraceSummaries(w, runs)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

// WriteThreadList renders thread list data in the requested format.
func WriteThreadList(w io.Writer, format string, threads []corethreads.ThreadData) error {
	switch format {
	case "json", "raw":
		return writeJSONThreadList(w, threads)
	case "pretty":
		return writePrettyThreadList(w, threads)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}

func writeJSONTraceMessages(w io.Writer, messages []coresingle.Message) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(messages)
}

func writeJSONThreadMessages(w io.Writer, messages []coresingle.ThreadMessage) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(messages)
}

func writeJSONTraceSummaries(w io.Writer, runs []coretraces.Summary) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(runs)
}

func writeJSONThreadList(w io.Writer, threads []corethreads.ThreadData) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(threads)
}

func writePrettyMessages(w io.Writer, messages []json.RawMessage, emptyMessage string) error {
	if len(messages) == 0 {
		fmt.Fprintln(w, emptyMessage)
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

func writePrettyTraceSummaries(w io.Writer, runs []coretraces.Summary) error {
	if len(runs) == 0 {
		fmt.Fprintln(w, "No traces found.")
		return nil
	}

	for _, run := range runs {
		if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", run.ID, run.Name, run.StartTime); err != nil {
			return err
		}
	}
	return nil
}

func writePrettyThreadList(w io.Writer, threads []corethreads.ThreadData) error {
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
