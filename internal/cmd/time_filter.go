// time_filter.go parses and validates list-command time filter flags.
package cmd

import (
	"fmt"
	"strings"
	"time"
)

const unsetLastNMinutes = -1

func parseStartTime(lastNMinutes int, since string, now func() time.Time) (string, error) {
	since = strings.TrimSpace(since)
	if lastNMinutes != unsetLastNMinutes && since != "" {
		return "", fmt.Errorf("--last-n-minutes and --since are mutually exclusive")
	}

	if lastNMinutes != unsetLastNMinutes {
		if lastNMinutes <= 0 {
			return "", fmt.Errorf("--last-n-minutes must be > 0")
		}
		return now().UTC().Add(-time.Duration(lastNMinutes) * time.Minute).Format(time.RFC3339), nil
	}

	if since == "" {
		return "", nil
	}

	parsed, err := time.Parse(time.RFC3339, since)
	if err != nil {
		return "", fmt.Errorf(
			"invalid --since value %q: expected RFC3339 timestamp (e.g., 2025-12-09T10:00:00Z)",
			since,
		)
	}
	return parsed.UTC().Format(time.RFC3339), nil
}
