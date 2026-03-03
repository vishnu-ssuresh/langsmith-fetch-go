package cmd

import (
	"strings"
	"testing"
)

func TestValidateOutputFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format string
	}{
		{name: "pretty", format: "pretty"},
		{name: "json", format: "json"},
		{name: "raw", format: "raw"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateOutputFormat(tc.format); err != nil {
				t.Fatalf("validateOutputFormat(%q) error = %v, want nil", tc.format, err)
			}
		})
	}
}

func TestValidateOutputFormat_Error(t *testing.T) {
	t.Parallel()

	err := validateOutputFormat("yaml")
	if err == nil || !strings.Contains(err.Error(), "--format must be one of") {
		t.Fatalf("validateOutputFormat() error = %v, want format validation error", err)
	}
}

func TestValidatePositiveIntFlag(t *testing.T) {
	t.Parallel()

	if err := validatePositiveIntFlag("limit", 1); err != nil {
		t.Fatalf("validatePositiveIntFlag() error = %v, want nil", err)
	}
	err := validatePositiveIntFlag("limit", 0)
	if err == nil || !strings.Contains(err.Error(), "--limit must be > 0") {
		t.Fatalf("validatePositiveIntFlag() error = %v, want limit validation error", err)
	}
}

func TestValidateMutuallyExclusiveStringFlags(t *testing.T) {
	t.Parallel()

	if err := validateMutuallyExclusiveStringFlags("file", "a.json", "dir", ""); err != nil {
		t.Fatalf("validateMutuallyExclusiveStringFlags() error = %v, want nil", err)
	}
	err := validateMutuallyExclusiveStringFlags("file", "a.json", "dir", "out")
	if err == nil || !strings.Contains(err.Error(), "--file and --dir are mutually exclusive") {
		t.Fatalf("validateMutuallyExclusiveStringFlags() error = %v, want exclusivity error", err)
	}
}
