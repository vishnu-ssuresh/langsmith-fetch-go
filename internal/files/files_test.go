// files_test.go validates filename sanitization, pattern expansion, and writes.
package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveFilename_ReplacesPlaceholders(t *testing.T) {
	t.Parallel()

	name, err := ResolveFilename("trace_{trace_id}_{index}", NameParams{
		TraceID: "abc-123",
		Index:   7,
	})
	if err != nil {
		t.Fatalf("ResolveFilename() error = %v", err)
	}
	if name != "trace_abc-123_7.json" {
		t.Fatalf("name = %q, want %q", name, "trace_abc-123_7.json")
	}
}

func TestResolveFilename_AppendsJSONExtension(t *testing.T) {
	t.Parallel()

	name, err := ResolveFilename("thread_{thread_id}", NameParams{ThreadID: "t-1"})
	if err != nil {
		t.Fatalf("ResolveFilename() error = %v", err)
	}
	if !strings.HasSuffix(name, ".json") {
		t.Fatalf("name = %q, want .json suffix", name)
	}
}

func TestSanitizeFilename(t *testing.T) {
	t.Parallel()

	got := SanitizeFilename(` bad:name/with*chars?.json `)
	if got != "bad_name_with_chars_.json" {
		t.Fatalf("SanitizeFilename() = %q, want %q", got, "bad_name_with_chars_.json")
	}
}

func TestWriteFile_CreatesParentDirs(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "out.json")
	if err := WriteFile(path, []byte(`{"ok":true}`)); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("file content = %q, want %q", string(data), `{"ok":true}`)
	}
}

func TestEnsureDir(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "a", "b", "c")
	if err := EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("path %q is not a directory", dir)
	}
}
