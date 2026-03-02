// root_test.go verifies root command behavior and dispatch errors.
package cmd

import (
	"bytes"
	"strings"
	"testing"

	"langsmith-fetch-go/internal/config"
)

func TestExecute_ThreadRequiresAPIKey(t *testing.T) {
	t.Parallel()

	err := Execute([]string{"thread"}, &bytes.Buffer{}, &bytes.Buffer{}, Deps{
		LoadConfig: func() config.Values { return config.Values{} },
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "LANGSMITH_API_KEY") {
		t.Fatalf("Execute() error = %v, want API key message", err)
	}
}

func TestExecute_ShowsUsageWithNoArgs(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := Execute(nil, &out, &bytes.Buffer{}, Deps{
		LoadConfig: func() config.Values { return config.Values{} },
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Fatalf("stdout = %q, want usage", out.String())
	}
}

func TestExecute_UnknownCommand(t *testing.T) {
	t.Parallel()

	err := Execute([]string{"unknown"}, &bytes.Buffer{}, &bytes.Buffer{}, Deps{
		LoadConfig: func() config.Values { return config.Values{APIKey: "test"} },
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("Execute() error = %v, want unknown command", err)
	}
}

func TestExecute_TraceCommandIsStubbed(t *testing.T) {
	t.Parallel()

	err := Execute([]string{"trace"}, &bytes.Buffer{}, &bytes.Buffer{}, Deps{
		LoadConfig: func() config.Values { return config.Values{APIKey: "test"} },
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "not implemented yet") {
		t.Fatalf("Execute() error = %v, want stub message", err)
	}
}

func TestExecute_ConfigShowDoesNotRequireAPIKey(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := Execute([]string{"config", "show"}, &out, &bytes.Buffer{}, Deps{
		LoadConfig: func() config.Values {
			return config.Values{
				ProjectName: "my-project",
			}
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "Current configuration:") {
		t.Fatalf("stdout = %q, want config output", out.String())
	}
}
