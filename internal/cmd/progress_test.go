// progress_test.go verifies stderr progress rendering behavior.
package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestProgressReporter_UpdateAndDone(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	reporter := newProgressReporter(&out, "threads", true)
	reporter.Update(0, 3)
	reporter.Update(1, 3)
	reporter.Update(3, 3)
	reporter.Done()

	got := out.String()
	if !strings.Contains(got, "Fetching threads 0/3") {
		t.Fatalf("output = %q, want initial progress", got)
	}
	if !strings.Contains(got, "Fetching threads 3/3") {
		t.Fatalf("output = %q, want final progress", got)
	}
}

func TestProgressReporter_Disabled(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	reporter := newProgressReporter(&out, "traces", false)
	reporter.Update(1, 2)
	reporter.Done()

	if out.Len() != 0 {
		t.Fatalf("output = %q, want empty when disabled", out.String())
	}
}
