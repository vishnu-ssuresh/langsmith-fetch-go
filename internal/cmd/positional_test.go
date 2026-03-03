package cmd

import (
	"strings"
	"testing"
)

func TestPopLeadingPositionalArg(t *testing.T) {
	t.Parallel()

	leading, rest := popLeadingPositionalArg([]string{"trace-123", "--format", "json"})
	if leading != "trace-123" {
		t.Fatalf("leading = %q, want %q", leading, "trace-123")
	}
	if len(rest) != 2 || rest[0] != "--format" {
		t.Fatalf("rest = %#v, want [--format json]", rest)
	}

	leading, rest = popLeadingPositionalArg([]string{"--trace-id", "trace-123"})
	if leading != "" {
		t.Fatalf("leading = %q, want empty", leading)
	}
	if len(rest) != 2 || rest[0] != "--trace-id" {
		t.Fatalf("rest = %#v, want unchanged args", rest)
	}
}

func TestResolveRequiredIDFromArgs(t *testing.T) {
	t.Parallel()

	id, err := resolveRequiredIDFromArgs("from-flag", "from-leading", nil, "trace-id")
	if err != nil {
		t.Fatalf("resolveRequiredIDFromArgs() error = %v", err)
	}
	if id != "from-flag" {
		t.Fatalf("id = %q, want %q", id, "from-flag")
	}

	id, err = resolveRequiredIDFromArgs("", "from-leading", []string{}, "trace-id")
	if err != nil {
		t.Fatalf("resolveRequiredIDFromArgs() error = %v", err)
	}
	if id != "from-leading" {
		t.Fatalf("id = %q, want %q", id, "from-leading")
	}

	id, err = resolveRequiredIDFromArgs("", "", []string{"from-rest"}, "trace-id")
	if err != nil {
		t.Fatalf("resolveRequiredIDFromArgs() error = %v", err)
	}
	if id != "from-rest" {
		t.Fatalf("id = %q, want %q", id, "from-rest")
	}
}

func TestResolveRequiredIDFromArgs_MissingAndExtraArgs(t *testing.T) {
	t.Parallel()

	_, err := resolveRequiredIDFromArgs("", "", nil, "trace-id")
	if err == nil || !strings.Contains(err.Error(), "--trace-id is required") {
		t.Fatalf("resolveRequiredIDFromArgs() error = %v, want missing id error", err)
	}

	_, err = resolveRequiredIDFromArgs("", "", []string{"trace-1", "extra"}, "trace-id")
	if err == nil || !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Fatalf("resolveRequiredIDFromArgs() error = %v, want extra positional args error", err)
	}

	_, err = resolveRequiredIDFromArgs("trace-1", "", []string{"extra"}, "trace-id")
	if err == nil || !strings.Contains(err.Error(), "unexpected positional arguments") {
		t.Fatalf("resolveRequiredIDFromArgs() error = %v, want extra positional args error", err)
	}
}
