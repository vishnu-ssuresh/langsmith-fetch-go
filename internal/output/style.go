// style.go defines optional ANSI styling for interactive terminals.
package output

import (
	"io"
	"os"
	"strings"
)

const ansiReset = "\x1b[0m"

type renderStyle struct {
	enabled bool
}

func newRenderStyle(w io.Writer) renderStyle {
	if os.Getenv("NO_COLOR") != "" {
		return renderStyle{}
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return renderStyle{}
	}

	file, ok := w.(*os.File)
	if !ok {
		return renderStyle{}
	}
	info, err := file.Stat()
	if err != nil {
		return renderStyle{}
	}
	if (info.Mode() & os.ModeCharDevice) == 0 {
		return renderStyle{}
	}
	return renderStyle{enabled: true}
}

func (s renderStyle) Heading(value string) string {
	return s.wrap("1;36", value) // bold cyan
}

func (s renderStyle) Dim(value string) string {
	return s.wrap("2", value)
}

func (s renderStyle) Role(role string) string {
	switch role {
	case "assistant":
		return s.wrap("32", role) // green
	case "user":
		return s.wrap("34", role) // blue
	case "system":
		return s.wrap("33", role) // yellow
	case "tool":
		return s.wrap("36", role) // cyan
	default:
		return role
	}
}

func (s renderStyle) wrap(code string, value string) string {
	if !s.enabled || value == "" {
		return value
	}
	return "\x1b[" + code + "m" + value + ansiReset
}
