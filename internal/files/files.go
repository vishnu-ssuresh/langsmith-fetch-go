// files.go provides safe filename and file-write helpers for CLI output.
package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NameParams contains placeholder values for filename pattern expansion.
type NameParams struct {
	ID       string
	TraceID  string
	ThreadID string
	Index    int
}

// EnsureDir creates a directory tree if it does not exist.
func EnsureDir(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("files: directory path is required")
	}
	return os.MkdirAll(path, 0o755)
}

// ResolveFilename expands known placeholders and sanitizes the result.
//
// Supported placeholders:
// - {id}
// - {trace_id}
// - {thread_id}
// - {index}
func ResolveFilename(pattern string, params NameParams) (string, error) {
	if strings.TrimSpace(pattern) == "" {
		return "", fmt.Errorf("files: filename pattern is required")
	}

	name := pattern
	name = strings.ReplaceAll(name, "{id}", params.ID)
	name = strings.ReplaceAll(name, "{trace_id}", params.TraceID)
	name = strings.ReplaceAll(name, "{thread_id}", params.ThreadID)
	name = strings.ReplaceAll(name, "{index}", fmt.Sprintf("%d", params.Index))

	safe := SanitizeFilename(name)
	if safe == "" {
		return "", fmt.Errorf("files: resolved filename is empty")
	}
	if !strings.HasSuffix(strings.ToLower(safe), ".json") {
		safe += ".json"
	}
	return safe, nil
}

// SanitizeFilename strips unsafe characters for cross-platform file names.
func SanitizeFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return ""
	}

	var b strings.Builder
	b.Grow(len(filename))
	for _, r := range filename {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}

	safe := strings.TrimSpace(b.String())
	safe = strings.Trim(safe, ". ")
	if len(safe) > 255 {
		safe = safe[:255]
	}
	return safe
}

// WriteFile writes bytes to a file path, creating parent directories as needed.
func WriteFile(path string, data []byte) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("files: file path is required")
	}

	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("files: create parent directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("files: write file: %w", err)
	}
	return nil
}
