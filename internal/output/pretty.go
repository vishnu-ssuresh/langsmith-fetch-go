// pretty.go contains human-friendly renderers used by --format pretty.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	corethreads "langsmith-fetch-go/internal/core/threads"
	coretraces "langsmith-fetch-go/internal/core/traces"
)

type prettyMessageRenderOptions struct {
	Heading      string
	Indent       string
	EmptyMessage string
}

func writePrettyMessages(w io.Writer, messages []json.RawMessage, opts prettyMessageRenderOptions) error {
	style := newRenderStyle(w)
	if len(messages) == 0 {
		if opts.EmptyMessage == "" {
			opts.EmptyMessage = "No messages found."
		}
		_, err := fmt.Fprintln(w, opts.Indent+style.Dim(opts.EmptyMessage))
		return err
	}

	if opts.Heading != "" {
		heading := fmt.Sprintf("%s (%d)", opts.Heading, len(messages))
		if _, err := fmt.Fprintf(w, "%s%s\n", opts.Indent, style.Heading(heading)); err != nil {
			return err
		}
	}

	for i, message := range messages {
		rendered := renderMessageSummary(message)
		role := normalizeRole(rendered.Role)
		if _, err := fmt.Fprintf(w, "%s%d. %s\n", opts.Indent, i+1, style.Role(role)); err != nil {
			return err
		}
		for _, line := range splitLines(rendered.Content) {
			if _, err := fmt.Fprintf(w, "%s   %s\n", opts.Indent, line); err != nil {
				return err
			}
		}
	}
	return nil
}

type renderedMessage struct {
	Role    string
	Content string
}

func renderMessageSummary(raw json.RawMessage) renderedMessage {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return renderedMessage{
			Role:    "message",
			Content: "(empty)",
		}
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return renderedMessage{
			Role:    "message",
			Content: trimmed,
		}
	}

	role := pickMessageRole(payload)
	content := pickMessageContent(payload)
	if content == "" {
		content = compactOrRaw(raw)
	}
	return renderedMessage{
		Role:    role,
		Content: content,
	}
}

func pickMessageRole(payload map[string]any) string {
	if role := pickString(payload, "role", "sender", "type", "name"); role != "" {
		return role
	}
	if kwargs, ok := payload["kwargs"].(map[string]any); ok {
		if role := pickString(kwargs, "role", "sender", "type", "name"); role != "" {
			return role
		}
	}
	return "message"
}

func normalizeRole(role string) string {
	role = strings.TrimSpace(strings.ToLower(role))
	switch role {
	case "", "msg":
		return "message"
	case "human", "user_message":
		return "user"
	case "ai", "assistant_message":
		return "assistant"
	case "system_message":
		return "system"
	default:
		return role
	}
}

func pickMessageContent(payload map[string]any) string {
	if content := pickMessageContentFromMap(payload); content != "" {
		return content
	}
	if kwargs, ok := payload["kwargs"].(map[string]any); ok {
		if content := pickMessageContentFromMap(kwargs); content != "" {
			return content
		}
	}
	return ""
}

func pickMessageContentFromMap(payload map[string]any) string {
	for _, key := range []string{"content", "text", "value", "message"} {
		value, ok := payload[key]
		if !ok {
			continue
		}
		rendered := renderAnyValue(value)
		if rendered != "" {
			return rendered
		}
	}
	return ""
}

func renderAnyValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case []any:
		lines := make([]string, 0, len(v))
		for _, item := range v {
			rendered := renderAnyValue(item)
			if rendered == "" {
				continue
			}
			lines = append(lines, rendered)
		}
		return strings.Join(lines, "\n")
	case map[string]any:
		if text := pickString(v, "text", "content", "value"); text != "" {
			return text
		}
		return marshalCompact(v)
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return marshalCompact(v)
	}
}

func pickString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}
		asString, ok := value.(string)
		if !ok {
			continue
		}
		asString = strings.TrimSpace(asString)
		if asString != "" {
			return asString
		}
	}
	return ""
}

func marshalCompact(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return compactOrRaw(data)
}

func compactOrRaw(data []byte) string {
	var out bytes.Buffer
	if err := json.Compact(&out, data); err != nil {
		return strings.TrimSpace(string(data))
	}
	return out.String()
}

func splitLines(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return []string{"(no content)"}
	}

	parts := strings.Split(trimmed, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(part)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return []string{"(no content)"}
	}
	return lines
}

func fallbackDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

func feedbackCountLabel(items int) string {
	return strconv.Itoa(items)
}

func statusLabel(run coretraces.Summary) string {
	if run.Metadata == nil {
		return "-"
	}
	return fallbackDash(run.Metadata.Status)
}

func writePrettyTraceSummaries(w io.Writer, runs []coretraces.Summary) error {
	style := newRenderStyle(w)
	if len(runs) == 0 {
		_, err := fmt.Fprintln(w, style.Dim("No traces found."))
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "TRACE ID\tNAME\tSTART TIME\tSTATUS\tFEEDBACK"); err != nil {
		return err
	}
	for _, run := range runs {
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			fallbackDash(run.ID),
			fallbackDash(run.Name),
			fallbackDash(run.StartTime),
			statusLabel(run),
			feedbackCountLabel(len(run.Feedback)),
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writePrettyThreadList(w io.Writer, threads []corethreads.ThreadData) error {
	style := newRenderStyle(w)
	if len(threads) == 0 {
		_, err := fmt.Fprintln(w, style.Dim("No threads found."))
		return err
	}

	for _, thread := range threads {
		title := fmt.Sprintf(
			"Thread %s (%d messages)",
			fallbackDash(thread.ThreadID),
			len(thread.Messages),
		)
		if _, err := fmt.Fprintln(w, style.Heading(title)); err != nil {
			return err
		}
		if err := writePrettyMessages(w, thread.Messages, prettyMessageRenderOptions{
			Heading:      "Messages",
			Indent:       "  ",
			EmptyMessage: "No messages in thread.",
		}); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}
