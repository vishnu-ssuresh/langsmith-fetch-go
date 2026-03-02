// time_filter_test.go verifies list-command time filter parsing behavior.
package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestParseStartTime_UnsetReturnsEmpty(t *testing.T) {
	t.Parallel()

	startTime, err := parseStartTime(unsetLastNMinutes, "", time.Now)
	if err != nil {
		t.Fatalf("parseStartTime() error = %v", err)
	}
	if startTime != "" {
		t.Fatalf("startTime = %q, want empty", startTime)
	}
}

func TestParseStartTime_MutuallyExclusive(t *testing.T) {
	t.Parallel()

	_, err := parseStartTime(30, "2025-12-09T10:00:00Z", time.Now)
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("parseStartTime() error = %v, want mutual exclusivity error", err)
	}
}

func TestParseStartTime_InvalidLastNMinutes(t *testing.T) {
	t.Parallel()

	_, err := parseStartTime(0, "", time.Now)
	if err == nil || !strings.Contains(err.Error(), "must be > 0") {
		t.Fatalf("parseStartTime() error = %v, want > 0 validation error", err)
	}
}

func TestParseStartTime_InvalidSince(t *testing.T) {
	t.Parallel()

	_, err := parseStartTime(unsetLastNMinutes, "not-a-time", time.Now)
	if err == nil || !strings.Contains(err.Error(), "expected RFC3339") {
		t.Fatalf("parseStartTime() error = %v, want RFC3339 validation error", err)
	}
}

func TestParseStartTime_SinceNormalizesToUTC(t *testing.T) {
	t.Parallel()

	startTime, err := parseStartTime(unsetLastNMinutes, "2025-12-09T10:00:00+02:00", time.Now)
	if err != nil {
		t.Fatalf("parseStartTime() error = %v", err)
	}
	if startTime != "2025-12-09T08:00:00Z" {
		t.Fatalf("startTime = %q, want %q", startTime, "2025-12-09T08:00:00Z")
	}
}

func TestParseStartTime_LastNMinutesUsesNow(t *testing.T) {
	t.Parallel()

	fixedNow := func() time.Time {
		return time.Date(2026, time.March, 2, 15, 4, 5, 0, time.UTC)
	}
	startTime, err := parseStartTime(30, "", fixedNow)
	if err != nil {
		t.Fatalf("parseStartTime() error = %v", err)
	}
	if startTime != "2026-03-02T14:34:05Z" {
		t.Fatalf("startTime = %q, want %q", startTime, "2026-03-02T14:34:05Z")
	}
}
